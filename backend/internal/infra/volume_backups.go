package infra

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const genericBackupsDir = "/generic-backups"

type VolumeBackup struct {
	ID          string     `json:"id"`
	VolumeName  string     `json:"volume_name"`
	Filename    string     `json:"filename"`
	SizeBytes   *int64     `json:"size_bytes,omitempty"`
	Status      string     `json:"status"`
	Error       string     `json:"error,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

func (s *Service) ListVolumeBackups(ctx context.Context, volumeName string) ([]VolumeBackup, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, volume_name, filename, size_bytes, status, error, started_at, completed_at
		FROM volume_backups WHERE volume_name = $1 ORDER BY started_at DESC
	`, volumeName)
	if err != nil {
		return nil, fmt.Errorf("listando backups: %w", err)
	}
	defer rows.Close()
	out := []VolumeBackup{}
	for rows.Next() {
		var b VolumeBackup
		if err := rows.Scan(&b.ID, &b.VolumeName, &b.Filename, &b.SizeBytes, &b.Status, &b.Error, &b.StartedAt, &b.CompletedAt); err != nil {
			return nil, fmt.Errorf("lendo backup: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// backupFilePath re-checa confinamento dentro de genericBackupsDir mesmo o
// único escritor (BackupVolume) já validando volumeName antes de chegar
// aqui — não explorável hoje, mas todo outro sink de path desse tipo no
// projeto (ver resolveBackupPath do lado Postgres) tem essa re-checagem;
// deixar esse de fora quebra o idioma e vira armadilha se um segundo
// escritor aparecer sem repetir a validação (achado de auditoria).
func backupFilePath(volumeName, filename string) (string, error) {
	joined := filepath.Join(genericBackupsDir, volumeName, filename)
	if !strings.HasPrefix(joined, genericBackupsDir+string(filepath.Separator)) {
		return "", fmt.Errorf("caminho de backup inválido")
	}
	return joined, nil
}

// BackupVolume sobe um snapshot .tar.gz do volume inteiro — via o mesmo
// container auxiliar efêmero do file manager (withVolumeHelper), baixando
// a raiz montada como tar (API de archive, sem exec) e comprimindo em
// streaming direto pro disco, sem bufferizar o volume inteiro em memória.
// Síncrono de propósito (mesmo padrão de BuildFromDockerfile) — volume
// grande deixa a requisição demorada, aceitável pro MVP desse recurso.
func (s *Service) BackupVolume(ctx context.Context, volumeName string) (*VolumeBackup, error) {
	if volumeName == "" {
		return nil, fmt.Errorf("volume é obrigatório")
	}
	// Precisa validar ANTES de qualquer uso em path de arquivo — volumeName
	// vira segmento de diretório logo abaixo (dir/path), e withVolumeHelper
	// só valida depois, tarde demais pra evitar o MkdirAll/os.Create com um
	// "../../etc/algo" não sanitizado.
	if err := validateVolumeName(volumeName); err != nil {
		return nil, err
	}
	filename := fmt.Sprintf("%s_%s.tar.gz", volumeName, time.Now().UTC().Format("20060102-150405"))
	dir := filepath.Join(genericBackupsDir, volumeName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("criando diretório de backup: %w", err)
	}
	path, err := backupFilePath(volumeName, filename)
	if err != nil {
		return nil, err
	}

	var backupID string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO volume_backups (volume_name, filename, status) VALUES ($1, $2, 'running')
		RETURNING id::text
	`, volumeName, filename).Scan(&backupID)
	if err != nil {
		return nil, fmt.Errorf("registrando backup: %w", err)
	}

	runErr := s.withVolumeHelper(ctx, volumeName, func(helperID string) error {
		reader, _, dlErr := s.docker.DownloadFromContainer(ctx, helperID, volumeMountPoint)
		if dlErr != nil {
			return dlErr
		}
		defer reader.Close()

		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("criando arquivo de backup: %w", err)
		}
		defer f.Close()

		gz := gzip.NewWriter(f)
		defer gz.Close()

		if _, err := io.Copy(gz, reader); err != nil {
			return fmt.Errorf("gravando backup: %w", err)
		}
		return nil
	})

	if runErr != nil {
		os.Remove(path)
		_, _ = s.pool.Exec(ctx, `UPDATE volume_backups SET status = 'failed', error = $2, completed_at = now() WHERE id = $1`, backupID, runErr.Error())
		return nil, runErr
	}

	var sizeBytes int64
	if info, err := os.Stat(path); err == nil {
		sizeBytes = info.Size()
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE volume_backups SET status = 'completed', size_bytes = $2, completed_at = now() WHERE id = $1
	`, backupID, sizeBytes); err != nil {
		return nil, fmt.Errorf("atualizando registro de backup: %w", err)
	}

	return s.getVolumeBackup(ctx, backupID)
}

func (s *Service) getVolumeBackup(ctx context.Context, id string) (*VolumeBackup, error) {
	var b VolumeBackup
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, volume_name, filename, size_bytes, status, error, started_at, completed_at
		FROM volume_backups WHERE id = $1
	`, id).Scan(&b.ID, &b.VolumeName, &b.Filename, &b.SizeBytes, &b.Status, &b.Error, &b.StartedAt, &b.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("lendo backup: %w", err)
	}
	return &b, nil
}

// DownloadVolumeBackup devolve o caminho real do arquivo pro handler
// streamar — mesma convenção de ResolvedHostPath (só o handler HTTP acessa
// filesystem direto, o service só resolve/valida).
func (s *Service) DownloadVolumeBackup(ctx context.Context, id string) (*VolumeBackup, string, error) {
	b, err := s.getVolumeBackup(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if b.Status != "completed" {
		return nil, "", fmt.Errorf("backup ainda não concluído")
	}
	path, err := backupFilePath(b.VolumeName, b.Filename)
	if err != nil {
		return nil, "", err
	}
	return b, path, nil
}

// RestoreVolumeBackup extrai um snapshot .tar.gz de volta num volume —
// createNew=true cria um volume novo do zero (nunca toca em dado
// existente, mesmo padrão de "criar banco novo" do restore de backup
// Postgres); createNew=false restaura POR CIMA de um volume já existente,
// limpando o conteúdo atual antes (irreversível, confirmação é
// responsabilidade da UI).
func (s *Service) RestoreVolumeBackup(ctx context.Context, backupID, targetVolumeName string, createNew bool) error {
	if targetVolumeName == "" {
		return fmt.Errorf("volume de destino é obrigatório")
	}
	if err := validateVolumeName(targetVolumeName); err != nil {
		return err
	}
	b, err := s.getVolumeBackup(ctx, backupID)
	if err != nil {
		return err
	}
	if b.Status != "completed" {
		return fmt.Errorf("backup ainda não concluído")
	}

	if createNew {
		if err := s.docker.CreateVolume(ctx, targetVolumeName); err != nil {
			return fmt.Errorf("criando volume de destino: %w", err)
		}
	}

	path, err := backupFilePath(b.VolumeName, b.Filename)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("abrindo arquivo de backup: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("descompactando backup: %w", err)
	}
	defer gz.Close()

	return s.withVolumeHelper(ctx, targetVolumeName, func(helperID string) error {
		if !createNew {
			if err := s.docker.ClearDirectoryInContainer(ctx, helperID, volumeMountPoint); err != nil {
				return fmt.Errorf("limpando volume de destino: %w", err)
			}
		}
		return s.docker.UploadArchiveToContainer(ctx, helperID, volumeMountPoint, gz)
	})
}

func (s *Service) DeleteVolumeBackup(ctx context.Context, id string) error {
	b, err := s.getVolumeBackup(ctx, id)
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM volume_backups WHERE id = $1`, id); err != nil {
		return fmt.Errorf("excluindo registro de backup: %w", err)
	}
	if path, err := backupFilePath(b.VolumeName, b.Filename); err == nil {
		os.Remove(path)
	}
	return nil
}
