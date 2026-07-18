package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func (r *Repo) UpdateName(ctx context.Context, id, name string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE servers SET name = $1, updated_at = now() WHERE id = $2`,
		name, id,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: já existe um servidor com esse nome", ErrValidation)
		}
		return fmt.Errorf("atualizando nome do servidor %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) UpdateResources(ctx context.Context, id string, res Resources, cfg PostgresConfig) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE servers SET
			cpu_cores = $1, memory_mb = $2, disk_gb = $3,
			max_connections = $4, shared_buffers_mb = $5, work_mem_mb = $6,
			maintenance_work_mem_mb = $7, effective_cache_size_mb = $8,
			preset = 'custom', updated_at = now()
		WHERE id = $9
	`,
		res.CPUCores, res.MemoryMB, res.DiskGB,
		cfg.MaxConnections, cfg.SharedBuffersMB, cfg.WorkMemMB,
		cfg.MaintenanceWorkMemMB, cfg.EffectiveCacheSizeMB,
		id,
	)
	if err != nil {
		return fmt.Errorf("atualizando recursos do servidor %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) UpdateHostPort(ctx context.Context, id string, hostPort int) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE servers SET host_port = $1, updated_at = now() WHERE id = $2`,
		hostPort, id,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: porta %d já está em uso por outro servidor", ErrValidation, hostPort)
		}
		return fmt.Errorf("atualizando porta do servidor %s: %w", id, err)
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

// MaxHostPort retorna a maior porta já usada (host_port OU pooler_host_port),
// pra alocação da próxima porta livre partir dali — as duas compartilham a
// mesma faixa, então precisam ser consideradas juntas pra nunca colidir.
// Retorna 0 se não houver nenhum servidor ainda.
func (r *Repo) MaxHostPort(ctx context.Context) (int, error) {
	var max int
	err := r.pool.QueryRow(ctx, `SELECT GREATEST(COALESCE(MAX(host_port), 0), COALESCE(MAX(pooler_host_port), 0)) FROM servers`).Scan(&max)
	if err != nil {
		return 0, fmt.Errorf("consultando maior host_port: %w", err)
	}
	return max, nil
}

// SetPooler grava o container pgbouncer recém-criado e habilita o pooling.
func (r *Repo) SetPooler(ctx context.Context, id, containerID, containerName string, hostPort int, poolMode string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE servers SET
			pooler_enabled = true, pooler_container_id = $1, pooler_container_name = $2,
			pooler_host_port = $3, pooler_pool_mode = $4, updated_at = now()
		WHERE id = $5
	`, containerID, containerName, hostPort, poolMode, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: porta %d já está em uso", ErrValidation, hostPort)
		}
		return fmt.Errorf("gravando pooler do servidor %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ClearPooler desliga o pooling e limpa o registro do container removido.
func (r *Repo) ClearPooler(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE servers SET
			pooler_enabled = false, pooler_container_id = '', pooler_container_name = '',
			pooler_host_port = NULL, updated_at = now()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("limpando pooler do servidor %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const selectColumns = `
	SELECT
		id, name, description, version, status, preset,
		cpu_cores, memory_mb, disk_gb,
		max_connections, shared_buffers_mb, work_mem_mb,
		maintenance_work_mem_mb, effective_cache_size_mb, log_min_duration_statement_ms,
		host_port, username, password_encrypted, database_name,
		container_id, container_name, volume_name,
		pooler_enabled, pooler_container_id, pooler_container_name,
		COALESCE(pooler_host_port, 0), pooler_pool_mode,
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
		&s.PoolerEnabled, &s.PoolerContainerID, &s.PoolerContainerName,
		&s.PoolerHostPort, &s.PoolerPoolMode,
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
