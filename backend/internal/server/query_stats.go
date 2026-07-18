package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// enableQueryStatsPreload é o caso especial de enablePreloadLib usado na
// criação do servidor (pg_stat_statements pré-carregado por padrão em todo
// servidor novo). Ver extensions.go pro caso genérico usado sob demanda.
func (s *Service) enableQueryStatsPreload(ctx context.Context, record *Server) error {
	return s.enablePreloadLib(ctx, record, "pg_stat_statements")
}

func containsLib(csv, lib string) bool {
	start := 0
	for i := 0; i <= len(csv); i++ {
		if i == len(csv) || csv[i] == ',' {
			if csv[start:i] == lib {
				return true
			}
			start = i + 1
		}
	}
	return false
}

func (s *Service) enableQueryStatsExtension(ctx context.Context, record *Server) error {
	conn, err := s.connectTo(ctx, record, record.DatabaseName)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_stat_statements"); err != nil {
		return fmt.Errorf("habilitando pg_stat_statements: %w", err)
	}
	return nil
}

// EnableQueryStats é a versão "sob demanda" pra servidores já existentes (criados
// antes de isso virar padrão, ou que tiveram a extensão desabilitada por fora).
// Reinicia o container — é o único jeito de shared_preload_libraries pegar valer.
func (s *Service) EnableQueryStats(ctx context.Context, id string) error {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return err
	}

	if err := s.enableQueryStatsPreload(ctx, record); err != nil {
		return err
	}
	if err := s.docker.RestartContainer(ctx, record.ContainerID); err != nil {
		return err
	}
	if err := s.docker.WaitHealthy(ctx, record.ContainerID, 60*time.Second); err != nil {
		return err
	}
	password, err := s.secretBox.Open(record.PasswordEncrypted)
	if err != nil {
		return err
	}
	if err := waitPostgresReady(ctx, record.ContainerName, record.Username, password, record.DatabaseName, 60*time.Second); err != nil {
		return err
	}
	return s.enableQueryStatsExtension(ctx, record)
}

type SlowQuery struct {
	QueryID       int64   `json:"query_id"`
	Query         string  `json:"query"`
	Calls         int64   `json:"calls"`
	TotalExecMs   float64 `json:"total_exec_ms"`
	MeanExecMs    float64 `json:"mean_exec_ms"`
	Rows          int64   `json:"rows"`
	CacheHitRatio float64 `json:"cache_hit_ratio"`
}

var slowQueryOrderBy = map[string]string{
	"total_time": "s.total_exec_time",
	"mean_time":  "s.mean_exec_time",
	"calls":      "s.calls",
}

// ListSlowQueries retorna (queries, disponível, erro). disponível=false (sem
// erro) quer dizer "pg_stat_statements não tá habilitado nesse banco ainda" —
// caso esperado, não é falha da plataforma.
func (s *Service) ListSlowQueries(ctx context.Context, id, database, orderBy string) ([]SlowQuery, bool, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, false, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close(ctx)

	orderCol, ok := slowQueryOrderBy[orderBy]
	if !ok {
		orderCol = slowQueryOrderBy["total_time"]
	}

	rows, err := conn.Query(ctx, fmt.Sprintf(`
		SELECT
			s.queryid,
			s.query,
			s.calls,
			s.total_exec_time,
			s.mean_exec_time,
			s.rows,
			CASE WHEN (s.shared_blks_hit + s.shared_blks_read) = 0 THEN 0
				ELSE s.shared_blks_hit::float8 / (s.shared_blks_hit + s.shared_blks_read)
			END
		FROM pg_stat_statements s
		JOIN pg_database d ON d.oid = s.dbid
		WHERE d.datname = $1
		ORDER BY %s DESC
		LIMIT 25
	`, orderCol), database)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" { // undefined_table
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("lendo pg_stat_statements: %w", err)
	}
	defer rows.Close()

	queries := make([]SlowQuery, 0)
	for rows.Next() {
		var q SlowQuery
		if err := rows.Scan(&q.QueryID, &q.Query, &q.Calls, &q.TotalExecMs, &q.MeanExecMs, &q.Rows, &q.CacheHitRatio); err != nil {
			return nil, true, fmt.Errorf("lendo query lenta: %w", err)
		}
		queries = append(queries, q)
	}
	return queries, true, rows.Err()
}

func (s *Service) ResetQueryStats(ctx context.Context, id, database string) error {
	return s.execDDL(ctx, id, database, "SELECT pg_stat_statements_reset()")
}
