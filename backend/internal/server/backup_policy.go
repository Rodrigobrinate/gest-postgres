package server

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var allowedBackupFrequencies = map[string]bool{"daily": true, "weekly": true}

type BackupPolicy struct {
	ID             string     `json:"id"`
	ServerID       string     `json:"server_id"`
	DatabaseName   string     `json:"database_name"`
	Storage        string     `json:"storage"`
	Frequency      string     `json:"frequency"` // daily | weekly
	Weekday        *int       `json:"weekday,omitempty"`
	TimeOfDay      string     `json:"time_of_day"` // "HH:MM"
	RetentionCount int        `json:"retention_count"`
	Enabled        bool       `json:"enabled"`
	LastRunAt      *time.Time `json:"last_run_at"`
	LastRunStatus  string     `json:"last_run_status"`
	LastRunError   string     `json:"last_run_error"`
	CreatedAt      time.Time  `json:"created_at"`
}

type CreateBackupPolicyInput struct {
	DatabaseName   string `json:"database_name"`
	Storage        string `json:"storage"`
	Frequency      string `json:"frequency"`
	Weekday        *int   `json:"weekday"`
	TimeOfDay      string `json:"time_of_day"`
	RetentionCount int    `json:"retention_count"`
}

func (s *Service) CreateBackupPolicy(ctx context.Context, serverID string, in CreateBackupPolicyInput) (*BackupPolicy, error) {
	if in.DatabaseName == "" {
		return nil, fmt.Errorf("%w: database é obrigatório", ErrValidation)
	}
	if !identRegex.MatchString(in.DatabaseName) {
		return nil, fmt.Errorf("%w: nome de database inválido", ErrValidation)
	}
	if _, err := s.storageByName(ctx, in.Storage); err != nil {
		return nil, err
	}
	if !allowedBackupFrequencies[in.Frequency] {
		return nil, fmt.Errorf("%w: frequência deve ser daily ou weekly", ErrValidation)
	}
	if _, _, err := parseTimeOfDay(in.TimeOfDay); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if in.Frequency == "weekly" && (in.Weekday == nil || *in.Weekday < 0 || *in.Weekday > 6) {
		return nil, fmt.Errorf("%w: weekday (0-6) é obrigatório pra frequência semanal", ErrValidation)
	}
	if in.RetentionCount <= 0 {
		return nil, fmt.Errorf("%w: retention_count deve ser positivo", ErrValidation)
	}
	if _, err := s.repo.Get(ctx, serverID); err != nil {
		return nil, err
	}

	var p BackupPolicy
	err := s.repo.pool.QueryRow(ctx, `
		INSERT INTO backup_policies (server_id, database_name, storage, frequency, weekday, time_of_day, retention_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, server_id, database_name, storage, frequency, weekday, time_of_day, retention_count, enabled, last_run_at, last_run_status, last_run_error, created_at
	`, serverID, in.DatabaseName, in.Storage, in.Frequency, in.Weekday, in.TimeOfDay, in.RetentionCount).Scan(
		&p.ID, &p.ServerID, &p.DatabaseName, &p.Storage, &p.Frequency, &p.Weekday, &p.TimeOfDay,
		&p.RetentionCount, &p.Enabled, &p.LastRunAt, &p.LastRunStatus, &p.LastRunError, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("criando política de backup: %w", err)
	}
	return &p, nil
}

func (s *Service) ListBackupPolicies(ctx context.Context, serverID string) ([]BackupPolicy, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT id, server_id, database_name, storage, frequency, weekday, time_of_day, retention_count, enabled, last_run_at, last_run_status, last_run_error, created_at
		FROM backup_policies WHERE server_id = $1 ORDER BY created_at DESC
	`, serverID)
	if err != nil {
		return nil, fmt.Errorf("listando políticas de backup: %w", err)
	}
	defer rows.Close()

	out := make([]BackupPolicy, 0)
	for rows.Next() {
		var p BackupPolicy
		if err := rows.Scan(
			&p.ID, &p.ServerID, &p.DatabaseName, &p.Storage, &p.Frequency, &p.Weekday, &p.TimeOfDay,
			&p.RetentionCount, &p.Enabled, &p.LastRunAt, &p.LastRunStatus, &p.LastRunError, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("lendo política de backup: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) DeleteBackupPolicy(ctx context.Context, serverID, policyID string) error {
	tag, err := s.repo.pool.Exec(ctx, `DELETE FROM backup_policies WHERE id = $1 AND server_id = $2`, policyID, serverID)
	if err != nil {
		return fmt.Errorf("excluindo política de backup: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) SetBackupPolicyEnabled(ctx context.Context, serverID, policyID string, enabled bool) error {
	tag, err := s.repo.pool.Exec(ctx,
		`UPDATE backup_policies SET enabled = $1, updated_at = now() WHERE id = $2 AND server_id = $3`,
		enabled, policyID, serverID,
	)
	if err != nil {
		return fmt.Errorf("atualizando política de backup: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RunBackupPolicyNow dispara a política imediatamente (botão "rodar agora"),
// fora do agendamento normal — mesma função que o sweep chama, só que sob
// pedido explícito.
func (s *Service) RunBackupPolicyNow(ctx context.Context, policyID string) error {
	p, err := s.getBackupPolicy(ctx, policyID)
	if err != nil {
		return err
	}
	return s.runPolicyOnce(ctx, p)
}

func (s *Service) getBackupPolicy(ctx context.Context, policyID string) (*BackupPolicy, error) {
	var p BackupPolicy
	err := s.repo.pool.QueryRow(ctx, `
		SELECT id, server_id, database_name, storage, frequency, weekday, time_of_day, retention_count, enabled, last_run_at, last_run_status, last_run_error, created_at
		FROM backup_policies WHERE id = $1
	`, policyID).Scan(
		&p.ID, &p.ServerID, &p.DatabaseName, &p.Storage, &p.Frequency, &p.Weekday, &p.TimeOfDay,
		&p.RetentionCount, &p.Enabled, &p.LastRunAt, &p.LastRunStatus, &p.LastRunError, &p.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("lendo política de backup: %w", err)
	}
	return &p, nil
}

// runPolicyOnce cria o registro de backup, roda o dump SÍNCRONO (diferente
// do disparo manual via API, que roda em background) — o sweep precisa saber
// o resultado na hora pra já aplicar a retenção em seguida, e uma execução
// agendada não tem uma requisição HTTP esperando resposta mesmo.
func (s *Service) runPolicyOnce(ctx context.Context, p *BackupPolicy) error {
	policyID := p.ID
	backup, err := s.insertBackup(ctx, p.ServerID, &policyID, p.DatabaseName, p.Storage)
	if err != nil {
		s.recordPolicyRun(ctx, p.ID, err)
		return err
	}

	runErr := s.performBackup(ctx, backup.ID)
	s.recordPolicyRun(ctx, p.ID, runErr)
	if runErr != nil {
		return runErr
	}

	s.applyBackupRetention(ctx, p)
	return nil
}

func (s *Service) recordPolicyRun(ctx context.Context, policyID string, runErr error) {
	status := "ok"
	errMsg := ""
	if runErr != nil {
		status = "error"
		errMsg = runErr.Error()
	}
	_, err := s.repo.pool.Exec(ctx, `
		UPDATE backup_policies SET last_run_at = now(), last_run_status = $1, last_run_error = $2, updated_at = now()
		WHERE id = $3
	`, status, errMsg, policyID)
	if err != nil {
		slog.Error("backup policy: falha salvando resultado da execução", "policy_id", policyID, "error", err)
	}
}

// applyBackupRetention mantém só os N backups mais recentes GERADOS POR ESSA
// POLÍTICA (backups manuais não contam pro limite de nenhuma política) —
// apaga do storage de verdade, não só o registro.
func (s *Service) applyBackupRetention(ctx context.Context, p *BackupPolicy) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT id FROM backups
		WHERE policy_id = $1 AND status = 'completed'
		ORDER BY started_at DESC
		OFFSET $2
	`, p.ID, p.RetentionCount)
	if err != nil {
		slog.Error("backup policy: falha listando backups pra retenção", "policy_id", p.ID, "error", err)
		return
	}
	var toDelete []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			toDelete = append(toDelete, id)
		}
	}
	rows.Close()

	for _, backupID := range toDelete {
		if err := s.DeleteBackup(ctx, p.ServerID, backupID); err != nil {
			slog.Warn("backup policy: falha aplicando retenção num backup antigo", "backup_id", backupID, "error", err)
		}
	}
}

// RunBackupSweep roda em background e dispara toda política habilitada cujo
// próximo horário agendado já passou — "cron básico" (diário/semanal +
// horário) do jeito descrito no MVP, sem precisar de parser de cron de
// verdade.
func (s *Service) RunBackupSweep(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepBackupPoliciesOnce(ctx)
		}
	}
}

func (s *Service) sweepBackupPoliciesOnce(ctx context.Context) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT id, server_id, database_name, storage, frequency, weekday, time_of_day, retention_count, enabled, last_run_at, last_run_status, last_run_error, created_at
		FROM backup_policies WHERE enabled = true
	`)
	if err != nil {
		slog.Error("backup policy: falha listando políticas", "error", err)
		return
	}
	var due []BackupPolicy
	for rows.Next() {
		var p BackupPolicy
		if err := rows.Scan(
			&p.ID, &p.ServerID, &p.DatabaseName, &p.Storage, &p.Frequency, &p.Weekday, &p.TimeOfDay,
			&p.RetentionCount, &p.Enabled, &p.LastRunAt, &p.LastRunStatus, &p.LastRunError, &p.CreatedAt,
		); err != nil {
			continue
		}
		base := p.CreatedAt
		if p.LastRunAt != nil {
			base = *p.LastRunAt
		}
		next, err := nextScheduledRun(p.Frequency, p.Weekday, p.TimeOfDay, base)
		if err == nil && !time.Now().UTC().Before(next) {
			due = append(due, p)
		}
	}
	rows.Close()

	for i := range due {
		if err := s.runPolicyOnce(ctx, &due[i]); err != nil {
			slog.Warn("backup policy: execução agendada falhou", "policy_id", due[i].ID, "error", err)
		}
	}
}

func parseTimeOfDay(v string) (hour, minute int, err error) {
	parts := strings.Split(v, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("horário deve estar no formato HH:MM")
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("hora inválida")
	}
	minute, err = strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("minuto inválido")
	}
	return hour, minute, nil
}

// nextScheduledRun calcula a próxima ocorrência ESTRITAMENTE depois de
// `after` — tudo em UTC (o container roda em UTC por padrão, sem
// configuração de timezone própria no MVP).
func nextScheduledRun(frequency string, weekday *int, timeOfDay string, after time.Time) (time.Time, error) {
	hour, minute, err := parseTimeOfDay(timeOfDay)
	if err != nil {
		return time.Time{}, err
	}
	after = after.UTC()

	if frequency == "daily" {
		candidate := time.Date(after.Year(), after.Month(), after.Day(), hour, minute, 0, 0, time.UTC)
		if !candidate.After(after) {
			candidate = candidate.AddDate(0, 0, 1)
		}
		return candidate, nil
	}

	if weekday == nil {
		return time.Time{}, fmt.Errorf("weekday é obrigatório pra frequência semanal")
	}
	for i := 0; i < 8; i++ {
		day := after.AddDate(0, 0, i)
		if int(day.Weekday()) != *weekday {
			continue
		}
		candidate := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, time.UTC)
		if candidate.After(after) {
			return candidate, nil
		}
	}
	return time.Time{}, fmt.Errorf("não foi possível calcular a próxima execução")
}
