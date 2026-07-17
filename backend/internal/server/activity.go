package server

import (
	"context"
	"fmt"
	"time"
)

type ActivitySession struct {
	PID             int32      `json:"pid"`
	Username        string     `json:"username"`
	Database        string     `json:"database"`
	ApplicationName string     `json:"application_name"`
	ClientAddr      string     `json:"client_addr"`
	State           string     `json:"state"`
	Query           string     `json:"query"`
	QueryStart      *time.Time `json:"query_start"`
	BackendStart    *time.Time `json:"backend_start"`
}

// Activity retorna o pg_stat_activity do banco indicado (default: o banco
// inicial do servidor). Sessões de outros bancos do mesmo cluster também
// aparecem — pg_stat_activity é cluster-wide, não por-database.
func (s *Service) Activity(ctx context.Context, id, database string) ([]ActivitySession, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT
			pid,
			COALESCE(usename, ''),
			COALESCE(datname, ''),
			COALESCE(application_name, ''),
			COALESCE(client_addr::text, ''),
			COALESCE(state, ''),
			COALESCE(query, ''),
			query_start,
			backend_start
		FROM pg_stat_activity
		WHERE backend_type = 'client backend'
		ORDER BY query_start DESC NULLS LAST
	`)
	if err != nil {
		return nil, fmt.Errorf("consultando pg_stat_activity: %w", err)
	}
	defer rows.Close()

	sessions := make([]ActivitySession, 0)
	for rows.Next() {
		var a ActivitySession
		if err := rows.Scan(
			&a.PID, &a.Username, &a.Database, &a.ApplicationName,
			&a.ClientAddr, &a.State, &a.Query, &a.QueryStart, &a.BackendStart,
		); err != nil {
			return nil, fmt.Errorf("lendo sessão: %w", err)
		}
		sessions = append(sessions, a)
	}
	return sessions, rows.Err()
}

// CancelBackend/TerminateBackend agem sobre uma sessão específica pelo PID.
// Cancel só interrompe a query atual (conexão continua); terminate derruba
// a conexão inteira.
func (s *Service) CancelBackend(ctx context.Context, id string, pid int32) error {
	return s.signalBackend(ctx, id, pid, "pg_cancel_backend")
}

func (s *Service) TerminateBackend(ctx context.Context, id string, pid int32) error {
	return s.signalBackend(ctx, id, pid, "pg_terminate_backend")
}

func (s *Service) signalBackend(ctx context.Context, id string, pid int32, fn string) error {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return err
	}
	conn, err := s.connectTo(ctx, record, "")
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	var ok bool
	if err := conn.QueryRow(ctx, fmt.Sprintf("SELECT %s($1)", fn), pid).Scan(&ok); err != nil {
		return fmt.Errorf("chamando %s: %w", fn, err)
	}
	if !ok {
		return fmt.Errorf("%w: sessão pid=%d não encontrada ou já encerrada", ErrValidation, pid)
	}
	return nil
}
