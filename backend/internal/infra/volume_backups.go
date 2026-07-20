package infra

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func backupFilePath(volumeName, filename string) string {
	return filepath.Join(genericBackupsDir, volumeName, filename)
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
	filename := fmt.Sprintf("%s_%s.tar.gz", volumeName, time.Now().UTC().Format("20060102-150405"))
	dir := filepath.Join(genericBackupsDir, volumeName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("criando diretório de backup: %w", err)
	}
	path := backupFilePath(volumeName, filename)

	var backupID string
	err := s.pool.QueryRow(ctx, `
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
	return b, backupFilePath(b.VolumeName, b.Filename), nil
}

func (s *Service) DeleteVolumeBackup(ctx context.Context, id string) error {
	b, err := s.getVolumeBackup(ctx, id)
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM volume_backups WHERE id = $1`, id); err != nil {
		return fmt.Errorf("excluindo registro de backup: %w", err)
	}
	os.Remove(backupFilePath(b.VolumeName, b.Filename))
	return nil
}
