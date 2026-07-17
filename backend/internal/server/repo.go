package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("servidor não encontrado")

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) Create(ctx context.Context, s *Server) error {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO servers (
			name, description, version, status, preset,
			cpu_cores, memory_mb, disk_gb,
			max_connections, shared_buffers_mb, work_mem_mb,
			maintenance_work_mem_mb, effective_cache_size_mb, log_min_duration_statement_ms,
			host_port, username, password_encrypted, database_name,
			container_id, container_name, volume_name
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9, $10, $11,
			$12, $13, $14,
			$15, $16, $17, $18,
			$19, $20, $21
		)
		RETURNING id, created_at, updated_at
	`,
		s.Name, s.Description, s.Version, s.Status, s.Preset,
		s.Resources.CPUCores, s.Resources.MemoryMB, s.Resources.DiskGB,
		s.Config.MaxConnections, s.Config.SharedBuffersMB, s.Config.WorkMemMB,
		s.Config.MaintenanceWorkMemMB, s.Config.EffectiveCacheSizeMB, s.Config.LogMinDurationStatementMs,
		s.HostPort, s.Username, s.PasswordEncrypted, s.DatabaseName,
		s.ContainerID, s.ContainerName, s.VolumeName,
	)
	if err := row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return fmt.Errorf("inserindo servidor: %w", err)
	}
	return nil
}

func (r *Repo) Get(ctx context.Context, id string) (*Server, error) {
	row := r.pool.QueryRow(ctx, selectColumns+` WHERE id = $1`, id)
	return scanServer(row)
}

func (r *Repo) List(ctx context.Context) ([]*Server, error) {
	rows, err := r.pool.Query(ctx, selectColumns+` ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listando servidores: %w", err)
	}
	defer rows.Close()

	var out []*Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repo) UpdateStatus(ctx context.Context, id string, status Status) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE servers SET status = $1, updated_at = now() WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("atualizando status do servidor %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) SetContainerID(ctx context.Context, id, containerID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE servers SET container_id = $1, updated_at = now() WHERE id = $2`,
		containerID, id,
	)
	if err != nil {
		return fmt.Errorf("atualizando container_id do servidor %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) UpdatePassword(ctx context.Context, id, passwordEncrypted string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE servers SET password_encrypted = $1, updated_at = now() WHERE id = $2`,
		passwordEncrypted, id,
	)
	if err != nil {
		return fmt.Errorf("atualizando senha do servidor %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM servers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("excluindo servidor %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MaxHostPort retorna a maior host_port já usada, pra alocação da próxima porta
// livre partir dali. Retorna 0 se não houver nenhum servidor ainda.
func (r *Repo) MaxHostPort(ctx context.Context) (int, error) {
	var max int
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(MAX(host_port), 0) FROM servers`).Scan(&max)
	if err != nil {
		return 0, fmt.Errorf("consultando maior host_port: %w", err)
	}
	return max, nil
}

const selectColumns = `
	SELECT
		id, name, description, version, status, preset,
		cpu_cores, memory_mb, disk_gb,
		max_connections, shared_buffers_mb, work_mem_mb,
		maintenance_work_mem_mb, effective_cache_size_mb, log_min_duration_statement_ms,
		host_port, username, password_encrypted, database_name,
		container_id, container_name, volume_name,
		created_at, updated_at
	FROM servers
`

type scanner interface {
	Scan(dest ...any) error
}

func scanServer(row scanner) (*Server, error) {
	s := &Server{}
	err := row.Scan(
		&s.ID, &s.Name, &s.Description, &s.Version, &s.Status, &s.Preset,
		&s.Resources.CPUCores, &s.Resources.MemoryMB, &s.Resources.DiskGB,
		&s.Config.MaxConnections, &s.Config.SharedBuffersMB, &s.Config.WorkMemMB,
		&s.Config.MaintenanceWorkMemMB, &s.Config.EffectiveCacheSizeMB, &s.Config.LogMinDurationStatementMs,
		&s.HostPort, &s.Username, &s.PasswordEncrypted, &s.DatabaseName,
		&s.ContainerID, &s.ContainerName, &s.VolumeName,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("lendo linha de servidor: %w", err)
	}
	return s, nil
}
