package server

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
)

var allowedBackupStorages = map[string]bool{"local": true, "gdrive": true}

type Backup struct {
	ID           string     `json:"id"`
	ServerID     string     `json:"server_id"`
	PolicyID     *string    `json:"policy_id,omitempty"`
	DatabaseName string     `json:"database_name"`
	Storage      string     `json:"storage"`
	Filename     string     `json:"filename"`
	SizeBytes    *int64     `json:"size_bytes,omitempty"`
	Status       string     `json:"status"`
	Error        string     `json:"error,omitempty"`
	GDriveFileID string     `json:"-"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// storageRef reconstrói a referência que BackupStorage.Open/Delete precisam
// pra achar o arquivo de novo — local não guarda isso em coluna própria
// porque dá pra recalcular sempre igual (server_id/filename); gdrive precisa
// do file ID de verdade, esse sim persistido.
func (b *Backup) storageRef() string {
	if b.Storage == "gdrive" {
		return b.GDriveFileID
	}
	return filepath.Join(b.ServerID, b.Filename)
}

func (s *Service) storageByName(ctx context.Context, name string) (BackupStorage, error) {
	if !allowedBackupStorages[name] {
		return nil, fmt.Errorf("%w: storage %q inválido", ErrValidation, name)
	}
	if name == "local" {
		return LocalStorage{}, nil
	}
	return s.gdriveStorage(ctx)
}

// CreateBackup dispara um pg_dump manual em background — devolve a linha
// logo com status "running" pra UI mostrar progresso, sem bloquear a
// requisição HTTP no tempo do dump (pode ser bem maior que um timeout HTTP
// razoável pra bancos grandes).
func (s *Service) CreateBackup(ctx context.Context, serverID, database, storage string) (*Backup, error) {
	if database == "" {
		return nil, fmt.Errorf("%w: database é obrigatório", ErrValidation)
	}
	// identRegex (não só "não vazio") fecha dois problemas de uma vez: (1)
	// database vira segmento de filename persistido e usado em
	// os.Remove/os.Stat depois — sem isso um "../../hostfiles/pwn" escapa
	// pro bind de host; (2) database vai direto em `pg_dump -d <database>`,
	// e libpq trata "-d" como conninfo completa se tiver "="/URI — um valor
	// tipo "dbname=x host=atacante.com" redireciona a conexão (com
	// PGPASSWORD no ambiente) pro host do atacante.
	if !identRegex.MatchString(database) {
		return nil, fmt.Errorf("%w: nome de database inválido", ErrValidation)
	}
	if _, err := s.storageByName(ctx, storage); err != nil {
		return nil, err
	}
	if _, err := s.getRunningServer(ctx, serverID); err != nil {
		return nil, err
	}

	backup, err := s.insertBackup(ctx, serverID, nil, database, storage)
	if err != nil {
		return nil, err
	}

	go func() {
		bgCtx := context.WithoutCancel(ctx)
		if err := s.performBackup(bgCtx, backup.ID); err != nil {
			slog.Warn("backup manual falhou", "backup_id", backup.ID, "error", err)
		}
	}()

	return backup, nil
}

func (s *Service) insertBackup(ctx context.Context, serverID string, policyID *string, database, storage string) (*Backup, error) {
	filename := fmt.Sprintf("%s_%s.dump", database, time.Now().UTC().Format("20060102T150405Z"))
	var b Backup
	err := s.repo.pool.QueryRow(ctx, `
		INSERT INTO backups (server_id, policy_id, database_name, storage, filename, status)
		VALUES ($1, $2, $3, $4, $5, 'running')
		RETURNING id, server_id, policy_id, database_name, storage, filename, size_bytes, status, error, gdrive_file_id, started_at, completed_at
	`, serverID, policyID, database, storage, filename).Scan(
		&b.ID, &b.ServerID, &b.PolicyID, &b.DatabaseName, &b.Storage, &b.Filename,
		&b.SizeBytes, &b.Status, &b.Error, &b.GDriveFileID, &b.StartedAt, &b.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("registrando backup: %w", err)
	}
	return &b, nil
}

// performBackup faz o trabalho de verdade: pg_dump pro arquivo temporário,
// Store no storage escolhido, atualiza a linha com o resultado. Chamada em
// background por CreateBackup (manual) e de forma síncrona pelo sweep de
// políticas agendadas (que precisa saber o resultado na hora pra aplicar
// retenção logo em seguida).
func (s *Service) performBackup(ctx context.Context, backupID string) error {
	b, record, err := s.getBackupAndServer(ctx, backupID)
	if err != nil {
		return err
	}

	runErr := s.runBackupOnce(ctx, b, record)

	if runErr != nil {
		s.markBackupFailed(ctx, backupID, runErr)
		return runErr
	}
	return nil
}

func (s *Service) runBackupOnce(ctx context.Context, b *Backup, record *Server) error {
	password, err := s.secretBox.Open(record.PasswordEncrypted)
	if err != nil {
		return fmt.Errorf("decifrando senha do servidor: %w", err)
	}

	scratchPath, err := newScratchFile()
	if err != nil {
		return err
	}

	if err := runPgDump(ctx, record, b.DatabaseName, password, scratchPath); err != nil {
		os.Remove(scratchPath)
		return err
	}

	storage, err := s.storageByName(ctx, b.Storage)
	if err != nil {
		os.Remove(scratchPath)
		return err
	}

	ref, sizeBytes, err := storage.Store(ctx, b.ServerID, b.Filename, scratchPath)
	if err != nil {
		return fmt.Errorf("salvando backup no storage %s: %w", b.Storage, err)
	}

	gdriveFileID := ""
	if b.Storage == "gdrive" {
		gdriveFileID = ref
	}

	_, updErr := s.repo.pool.Exec(ctx, `
		UPDATE backups SET status = 'completed', size_bytes = $1, gdrive_file_id = $2, completed_at = now()
		WHERE id = $3
	`, sizeBytes, gdriveFileID, b.ID)
	if updErr != nil {
		return fmt.Errorf("salvando resultado do backup: %w", updErr)
	}
	return nil
}

func (s *Service) markBackupFailed(ctx context.Context, backupID string, runErr error) {
	_, err := s.repo.pool.Exec(ctx, `
		UPDATE backups SET status = 'failed', error = $1, completed_at = now() WHERE id = $2
	`, runErr.Error(), backupID)
	if err != nil {
		slog.Error("backup: falha salvando status de erro", "backup_id", backupID, "error", err)
	}
}

func (s *Service) getBackupAndServer(ctx context.Context, backupID string) (*Backup, *Server, error) {
	b, err := s.getBackup(ctx, backupID)
	if err != nil {
		return nil, nil, err
	}
	record, err := s.getRunningServer(ctx, b.ServerID)
	if err != nil {
		return nil, nil, err
	}
	return b, record, nil
}

func (s *Service) getBackup(ctx context.Context, backupID string) (*Backup, error) {
	var b Backup
	err := s.repo.pool.QueryRow(ctx, `
		SELECT id, server_id, policy_id, database_name, storage, filename, size_bytes, status, error, gdrive_file_id, started_at, completed_at
		FROM backups WHERE id = $1
	`, backupID).Scan(
		&b.ID, &b.ServerID, &b.PolicyID, &b.DatabaseName, &b.Storage, &b.Filename,
		&b.SizeBytes, &b.Status, &b.Error, &b.GDriveFileID, &b.StartedAt, &b.CompletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("lendo backup: %w", err)
	}
	return &b, nil
}

func (s *Service) ListBackups(ctx context.Context, serverID string) ([]Backup, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT id, server_id, policy_id, database_name, storage, filename, size_bytes, status, error, gdrive_file_id, started_at, completed_at
		FROM backups WHERE server_id = $1 ORDER BY started_at DESC
	`, serverID)
	if err != nil {
		return nil, fmt.Errorf("listando backups: %w", err)
	}
	defer rows.Close()

	out := make([]Backup, 0)
	for rows.Next() {
		var b Backup
		if err := rows.Scan(
			&b.ID, &b.ServerID, &b.PolicyID, &b.DatabaseName, &b.Storage, &b.Filename,
			&b.SizeBytes, &b.Status, &b.Error, &b.GDriveFileID, &b.StartedAt, &b.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("lendo backup: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Service) DeleteBackup(ctx context.Context, serverID, backupID string) error {
	b, err := s.getBackup(ctx, backupID)
	if err != nil {
		return err
	}
	if b.ServerID != serverID {
		return ErrNotFound
	}
	if b.Status == "completed" {
		storage, err := s.storageByName(ctx, b.Storage)
		if err != nil {
			return err
		}
		if err := storage.Delete(ctx, b.storageRef()); err != nil {
			return err
		}
	}
	tag, err := s.repo.pool.Exec(ctx, `DELETE FROM backups WHERE id = $1`, backupID)
	if err != nil {
		return fmt.Errorf("excluindo registro do backup: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DownloadBackup devolve um caminho local pro arquivo (baixa do Drive antes,
// se for o caso) — quem chama é responsável por chamar cleanup() depois de
// terminar de ler.
func (s *Service) DownloadBackup(ctx context.Context, serverID, backupID string) (path, filename string, cleanup func(), err error) {
	b, err := s.getBackup(ctx, backupID)
	if err != nil {
		return "", "", nil, err
	}
	if b.ServerID != serverID {
		return "", "", nil, ErrNotFound
	}
	if b.Status != "completed" {
		return "", "", nil, fmt.Errorf("%w: backup ainda não terminou (status %q)", ErrValidation, b.Status)
	}
	storage, err := s.storageByName(ctx, b.Storage)
	if err != nil {
		return "", "", nil, err
	}
	p, cleanupFn, err := storage.Open(ctx, b.storageRef())
	if err != nil {
		return "", "", nil, err
	}
	return p, b.Filename, cleanupFn, nil
}

type RestoreBackupInput struct {
	TargetDatabase  string `json:"target_database"`
	CreateNew       bool   `json:"create_new"`
	NewDatabaseName string `json:"new_database_name"`
}

// RestoreBackup baixa o dump (se necessário) e roda pg_restore --clean contra
// o Postgres gerenciado. Dois modos: sobrescreve um database já existente
// (TargetDatabase) ou cria um novo do zero antes de restaurar nele — nunca
// mexe no database original nesse segundo caso.
func (s *Service) RestoreBackup(ctx context.Context, serverID, backupID string, in RestoreBackupInput) error {
	b, err := s.getBackup(ctx, backupID)
	if err != nil {
		return err
	}
	if b.ServerID != serverID {
		return ErrNotFound
	}
	if b.Status != "completed" {
		return fmt.Errorf("%w: backup ainda não terminou (status %q)", ErrValidation, b.Status)
	}

	record, err := s.getRunningServer(ctx, serverID)
	if err != nil {
		return err
	}

	targetDB := in.TargetDatabase
	if in.CreateNew {
		if !identRegex.MatchString(in.NewDatabaseName) {
			return fmt.Errorf("%w: nome de banco novo inválido", ErrValidation)
		}
		if err := s.CreateDatabase(ctx, serverID, in.NewDatabaseName); err != nil {
			return err
		}
		targetDB = in.NewDatabaseName
	} else if !identRegex.MatchString(targetDB) {
		return fmt.Errorf("%w: target_database inválido", ErrValidation)
	}

	storage, err := s.storageByName(ctx, b.Storage)
	if err != nil {
		return err
	}
	localPath, cleanup, err := storage.Open(ctx, b.storageRef())
	if err != nil {
		return err
	}
	defer cleanup()

	password, err := s.secretBox.Open(record.PasswordEncrypted)
	if err != nil {
		return fmt.Errorf("decifrando senha do servidor: %w", err)
	}

	return runPgRestore(ctx, record, targetDB, password, localPath)
}

// runPgDump/runPgRestore rodam DIRETO do container do backend (que já tem
// pg_dump/pg_restore instalados — ver Dockerfile), conectando via rede na
// porta 5432 do container gerenciado, nunca via `docker exec` — mesma
// decisão de segurança do resto do projeto (docker-socket-proxy nunca
// libera a categoria EXEC de propósito).
func runPgDump(ctx context.Context, record *Server, database, password, destPath string) error {
	cmd := exec.CommandContext(ctx, "pg_dump",
		"-h", record.ContainerName,
		"-p", "5432",
		"-U", record.Username,
		"-d", database,
		"-Fc",
		"-f", destPath,
	)
	cmd.Env = append(cmd.Environ(), "PGPASSWORD="+password)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: pg_dump falhou: %v: %s", ErrValidation, err, stderr.String())
	}
	return nil
}

func runPgRestore(ctx context.Context, record *Server, database, password, sourcePath string) error {
	cmd := exec.CommandContext(ctx, "pg_restore",
		"-h", record.ContainerName,
		"-p", "5432",
		"-U", record.Username,
		"-d", database,
		"--clean", "--if-exists", "--no-owner",
		sourcePath,
	)
	cmd.Env = append(cmd.Environ(), "PGPASSWORD="+password)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: pg_restore falhou: %v: %s", ErrValidation, err, stderr.String())
	}
	return nil
}
