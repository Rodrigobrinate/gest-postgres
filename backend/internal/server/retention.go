package server

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

type RetentionPolicy struct {
	ID                  string     `json:"id"`
	ServerID            string     `json:"server_id"`
	DatabaseName        string     `json:"database_name"`
	SchemaName          string     `json:"schema_name"`
	TableName           string     `json:"table_name"`
	DateColumn          string     `json:"date_column"`
	MaxAgeDays          int        `json:"max_age_days"`
	Action              string     `json:"action"` // "archive" | "delete"
	Enabled             bool       `json:"enabled"`
	LastRunAt           *time.Time `json:"last_run_at"`
	LastRunRowsAffected *int64     `json:"last_run_rows_affected"`
	LastRunError        string     `json:"last_run_error"`
	CreatedAt           time.Time  `json:"created_at"`
}

var allowedRetentionActions = map[string]bool{"archive": true, "delete": true}

type CreateRetentionPolicyInput struct {
	DatabaseName string `json:"database_name"`
	SchemaName   string `json:"schema_name"`
	TableName    string `json:"table_name"`
	DateColumn   string `json:"date_column"`
	MaxAgeDays   int    `json:"max_age_days"`
	Action       string `json:"action"`
}

func (s *Service) CreateRetentionPolicy(ctx context.Context, id string, in CreateRetentionPolicyInput) (*RetentionPolicy, error) {
	if !identRegex.MatchString(in.SchemaName) || !identRegex.MatchString(in.TableName) || !identRegex.MatchString(in.DateColumn) {
		return nil, fmt.Errorf("%w: schema/tabela/coluna inválidos", ErrValidation)
	}
	if !allowedRetentionActions[in.Action] {
		return nil, fmt.Errorf("%w: action deve ser archive ou delete", ErrValidation)
	}
	if in.MaxAgeDays <= 0 {
		return nil, fmt.Errorf("%w: max_age_days deve ser positivo", ErrValidation)
	}
	if in.DatabaseName == "" {
		return nil, fmt.Errorf("%w: database é obrigatório", ErrValidation)
	}
	if _, err := s.getRunningServer(ctx, id); err != nil {
		return nil, err
	}

	var p RetentionPolicy
	err := s.repo.pool.QueryRow(ctx, `
		INSERT INTO retention_policies (server_id, database_name, schema_name, table_name, date_column, max_age_days, action)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, server_id, database_name, schema_name, table_name, date_column, max_age_days, action, enabled, last_run_at, last_run_rows_affected, last_run_error, created_at
	`, id, in.DatabaseName, in.SchemaName, in.TableName, in.DateColumn, in.MaxAgeDays, in.Action).Scan(
		&p.ID, &p.ServerID, &p.DatabaseName, &p.SchemaName, &p.TableName, &p.DateColumn, &p.MaxAgeDays, &p.Action,
		&p.Enabled, &p.LastRunAt, &p.LastRunRowsAffected, &p.LastRunError, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("criando política de retenção: %w", err)
	}
	return &p, nil
}

func (s *Service) ListRetentionPolicies(ctx context.Context, id string) ([]RetentionPolicy, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT id, server_id, database_name, schema_name, table_name, date_column, max_age_days, action, enabled, last_run_at, last_run_rows_affected, last_run_error, created_at
		FROM retention_policies
		WHERE server_id = $1
		ORDER BY created_at DESC
	`, id)
	if err != nil {
		return nil, fmt.Errorf("listando políticas de retenção: %w", err)
	}
	defer rows.Close()

	out := make([]RetentionPolicy, 0)
	for rows.Next() {
		var p RetentionPolicy
		if err := rows.Scan(
			&p.ID, &p.ServerID, &p.DatabaseName, &p.SchemaName, &p.TableName, &p.DateColumn, &p.MaxAgeDays, &p.Action,
			&p.Enabled, &p.LastRunAt, &p.LastRunRowsAffected, &p.LastRunError, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("lendo política de retenção: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) DeleteRetentionPolicy(ctx context.Context, id, policyID string) error {
	tag, err := s.repo.pool.Exec(ctx, `DELETE FROM retention_policies WHERE id = $1 AND server_id = $2`, policyID, id)
	if err != nil {
		return fmt.Errorf("excluindo política de retenção: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) SetRetentionPolicyEnabled(ctx context.Context, id, policyID string, enabled bool) error {
	tag, err := s.repo.pool.Exec(ctx,
		`UPDATE retention_policies SET enabled = $1, updated_at = now() WHERE id = $2 AND server_id = $3`,
		enabled, policyID, id,
	)
	if err != nil {
		return fmt.Errorf("atualizando política de retenção: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RunRetentionPolicy executa a política agora (chamada manual ou pelo loop em
// background). Archive faz INSERT na tabela _archive (criada com a mesma
// estrutura via LIKE, se ainda não existir) seguido de DELETE, dentro da
// mesma transação — se o INSERT falhar nada é apagado.
func (s *Service) RunRetentionPolicy(ctx context.Context, policyID string) (int64, error) {
	var p RetentionPolicy
	err := s.repo.pool.QueryRow(ctx, `
		SELECT id, server_id, database_name, schema_name, table_name, date_column, max_age_days, action
		FROM retention_policies WHERE id = $1
	`, policyID).Scan(&p.ID, &p.ServerID, &p.DatabaseName, &p.SchemaName, &p.TableName, &p.DateColumn, &p.MaxAgeDays, &p.Action)
	if err != nil {
		return 0, fmt.Errorf("lendo política de retenção: %w", err)
	}

	rowsAffected, runErr := s.execRetentionPolicy(ctx, &p)

	errMsg := ""
	if runErr != nil {
		errMsg = runErr.Error()
	}
	_, updateErr := s.repo.pool.Exec(ctx, `
		UPDATE retention_policies
		SET last_run_at = now(), last_run_rows_affected = $1, last_run_error = $2, updated_at = now()
		WHERE id = $3
	`, rowsAffected, errMsg, p.ID)
	if updateErr != nil {
		slog.Error("retenção: falha salvando resultado da execução", "policy_id", p.ID, "error", updateErr)
	}

	return rowsAffected, runErr
}

func (s *Service) execRetentionPolicy(ctx context.Context, p *RetentionPolicy) (int64, error) {
	record, err := s.getRunningServer(ctx, p.ServerID)
	if err != nil {
		return 0, err
	}
	conn, err := s.connectTo(ctx, record, p.DatabaseName)
	if err != nil {
		return 0, err
	}
	defer conn.Close(ctx)

	table := pgx.Identifier{p.SchemaName, p.TableName}.Sanitize()
	dateCol := pgx.Identifier{p.DateColumn}.Sanitize()
	cutoff := fmt.Sprintf("now() - interval '%d days'", p.MaxAgeDays)

	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("abrindo transação: %w", err)
	}
	defer tx.Rollback(ctx)

	if p.Action == "archive" {
		archiveTable := pgx.Identifier{p.SchemaName, p.TableName + "_archive"}.Sanitize()
		createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (LIKE %s INCLUDING ALL)", archiveTable, table)
		if _, err := tx.Exec(ctx, createSQL); err != nil {
			return 0, fmt.Errorf("%w: criando tabela de arquivo: %v", ErrValidation, err)
		}
		insertSQL := fmt.Sprintf("INSERT INTO %s SELECT * FROM %s WHERE %s < %s", archiveTable, table, dateCol, cutoff)
		if _, err := tx.Exec(ctx, insertSQL); err != nil {
			return 0, fmt.Errorf("%w: arquivando linhas: %v", ErrValidation, err)
		}
	}

	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE %s < %s", table, dateCol, cutoff)
	tag, err := tx.Exec(ctx, deleteSQL)
	if err != nil {
		return 0, fmt.Errorf("%w: apagando linhas antigas: %v", ErrValidation, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commitando: %w", err)
	}

	return tag.RowsAffected(), nil
}

// RunRetentionSweep roda em background (chamado uma vez no main) e executa
// toda política habilitada que não roda há mais de 24h — a versão "agendada
// simples" sem precisar de um cron de verdade.
func (s *Service) RunRetentionSweep(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepRetentionPoliciesOnce(ctx)
		}
	}
}

func (s *Service) sweepRetentionPoliciesOnce(ctx context.Context) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT id FROM retention_policies
		WHERE enabled = true AND (last_run_at IS NULL OR last_run_at < now() - interval '24 hours')
	`)
	if err != nil {
		slog.Error("retenção: falha listando políticas pendentes", "error", err)
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, policyID := range ids {
		if _, err := s.RunRetentionPolicy(ctx, policyID); err != nil {
			slog.Warn("retenção: execução automática falhou", "policy_id", policyID, "error", err)
		}
	}
}
