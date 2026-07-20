package server

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type AlertRule struct {
	ID              string     `json:"id"`
	ServerID        string     `json:"server_id"`
	Metric          string     `json:"metric"`
	Threshold       float64    `json:"threshold"`
	WebhookURL      string     `json:"webhook_url,omitempty"`
	ChannelID       string     `json:"channel_id,omitempty"`
	Enabled         bool       `json:"enabled"`
	LastTriggeredAt *time.Time `json:"last_triggered_at"`
	LastValue       *float64   `json:"last_value"`
	CreatedAt       time.Time  `json:"created_at"`
}

var allowedAlertMetrics = map[string]string{
	"connections_pct":            "% de conexões em uso (vs max_connections)",
	"disk_pct":                   "% de disco usado (vs preset do container)",
	"long_running_query_seconds": "segundos de execução da query ativa mais longa",
	"deadlocks":                  "novos deadlocks desde a última checagem",
}

type CreateAlertRuleInput struct {
	Metric     string  `json:"metric"`
	Threshold  float64 `json:"threshold"`
	WebhookURL string  `json:"webhook_url,omitempty"`
	ChannelID  string  `json:"channel_id,omitempty"`
}

func (s *Service) CreateAlertRule(ctx context.Context, id string, in CreateAlertRuleInput) (*AlertRule, error) {
	if _, ok := allowedAlertMetrics[in.Metric]; !ok {
		return nil, fmt.Errorf("%w: métrica inválida", ErrValidation)
	}
	if in.WebhookURL == "" && in.ChannelID == "" {
		return nil, fmt.Errorf("%w: escolha um canal salvo ou informe uma webhook_url", ErrValidation)
	}
	if in.WebhookURL != "" {
		if err := validateWebhookURL(in.WebhookURL); err != nil {
			return nil, err
		}
	}
	if in.Threshold <= 0 {
		return nil, fmt.Errorf("%w: threshold deve ser positivo", ErrValidation)
	}
	if _, err := s.getRunningServer(ctx, id); err != nil {
		return nil, err
	}

	var channelID *string
	if in.ChannelID != "" {
		channelID = &in.ChannelID
	}

	var a AlertRule
	err := s.repo.pool.QueryRow(ctx, `
		INSERT INTO alert_rules (server_id, metric, threshold, webhook_url, channel_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, server_id, metric, threshold, webhook_url, coalesce(channel_id::text, ''), enabled, last_triggered_at, last_value, created_at
	`, id, in.Metric, in.Threshold, in.WebhookURL, channelID).Scan(
		&a.ID, &a.ServerID, &a.Metric, &a.Threshold, &a.WebhookURL, &a.ChannelID, &a.Enabled, &a.LastTriggeredAt, &a.LastValue, &a.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("criando regra de alerta: %w", err)
	}
	return &a, nil
}

func (s *Service) ListAlertRules(ctx context.Context, id string) ([]AlertRule, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT id, server_id, metric, threshold, webhook_url, coalesce(channel_id::text, ''), enabled, last_triggered_at, last_value, created_at
		FROM alert_rules WHERE server_id = $1 ORDER BY created_at DESC
	`, id)
	if err != nil {
		return nil, fmt.Errorf("listando regras de alerta: %w", err)
	}
	defer rows.Close()

	out := make([]AlertRule, 0)
	for rows.Next() {
		var a AlertRule
		if err := rows.Scan(&a.ID, &a.ServerID, &a.Metric, &a.Threshold, &a.WebhookURL, &a.ChannelID, &a.Enabled, &a.LastTriggeredAt, &a.LastValue, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("lendo regra de alerta: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Service) DeleteAlertRule(ctx context.Context, id, ruleID string) error {
	tag, err := s.repo.pool.Exec(ctx, `DELETE FROM alert_rules WHERE id = $1 AND server_id = $2`, ruleID, id)
	if err != nil {
		return fmt.Errorf("excluindo regra de alerta: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) SetAlertRuleEnabled(ctx context.Context, id, ruleID string, enabled bool) error {
	tag, err := s.repo.pool.Exec(ctx,
		`UPDATE alert_rules SET enabled = $1 WHERE id = $2 AND server_id = $3`,
		enabled, ruleID, id,
	)
	if err != nil {
		return fmt.Errorf("atualizando regra de alerta: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// alertMetricValue calcula o valor atual de uma métrica pra um servidor.
// Reaproveita as mesmas fontes já usadas em health.go/capacity — não duplica
// a query, só pega o número final.
func (s *Service) alertMetricValue(ctx context.Context, record *Server, metric string) (float64, error) {
	switch metric {
	case "connections_pct":
		conn, err := s.connectTo(ctx, record, "")
		if err != nil {
			return 0, err
		}
		defer conn.Close(ctx)
		var used, max int
		err = conn.QueryRow(ctx, `
			SELECT
				(SELECT count(*) FROM pg_stat_activity WHERE backend_type = 'client backend'),
				(SELECT setting::int FROM pg_settings WHERE name = 'max_connections')
		`).Scan(&used, &max)
		if err != nil || max == 0 {
			return 0, err
		}
		return float64(used) / float64(max) * 100, nil

	case "disk_pct":
		forecast, err := s.GetCapacityForecast(ctx, record.ID)
		if err != nil || forecast.DiskLimitMB == 0 {
			return 0, err
		}
		return forecast.CurrentDiskMB / forecast.DiskLimitMB * 100, nil

	case "long_running_query_seconds":
		conn, err := s.connectTo(ctx, record, "")
		if err != nil {
			return 0, err
		}
		defer conn.Close(ctx)
		var seconds float64
		err = conn.QueryRow(ctx, `
			SELECT COALESCE(MAX(EXTRACT(EPOCH FROM (now() - query_start))), 0)
			FROM pg_stat_activity
			WHERE state = 'active' AND backend_type = 'client backend'
		`).Scan(&seconds)
		return seconds, err

	case "deadlocks":
		conn, err := s.connectTo(ctx, record, "")
		if err != nil {
			return 0, err
		}
		defer conn.Close(ctx)
		var count int64
		err = conn.QueryRow(ctx, `SELECT COALESCE(sum(deadlocks), 0) FROM pg_stat_database`).Scan(&count)
		return float64(count), err

	default:
		return 0, fmt.Errorf("métrica desconhecida: %s", metric)
	}
}

type webhookPayload struct {
	ServerID    string    `json:"server_id"`
	ServerName  string    `json:"server_name"`
	Metric      string    `json:"metric"`
	Description string    `json:"description"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
	TriggeredAt time.Time `json:"triggered_at"`
}

const alertCooldown = 15 * time.Minute

// RunAlertSweep roda em background e checa toda regra habilitada a cada
// `interval` — dispara o webhook se o valor atual passar do threshold e a
// regra não tiver disparado nos últimos 15min (evita spam).
func (s *Service) RunAlertSweep(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepAlertRulesOnce(ctx)
		}
	}
}

func (s *Service) sweepAlertRulesOnce(ctx context.Context) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT ar.id, ar.server_id, ar.metric, ar.threshold, ar.webhook_url, coalesce(ar.channel_id::text, ''),
		       ar.last_triggered_at, ar.last_deadlock_count, s.name
		FROM alert_rules ar
		JOIN servers s ON s.id = ar.server_id
		WHERE ar.enabled = true AND s.status = 'running'
	`)
	if err != nil {
		slog.Error("alertas: falha listando regras", "error", err)
		return
	}

	type ruleRow struct {
		id, serverID, metric, webhookURL, channelID, serverName string
		threshold                                               float64
		lastTriggeredAt                                         *time.Time
		lastDeadlockCount                                       int64
	}
	var rules []ruleRow
	for rows.Next() {
		var rr ruleRow
		if err := rows.Scan(&rr.id, &rr.serverID, &rr.metric, &rr.threshold, &rr.webhookURL, &rr.channelID, &rr.lastTriggeredAt, &rr.lastDeadlockCount, &rr.serverName); err != nil {
			continue
		}
		rules = append(rules, rr)
	}
	rows.Close()

	for _, rr := range rules {
		if rr.lastTriggeredAt != nil && time.Since(*rr.lastTriggeredAt) < alertCooldown {
			continue
		}
		record, err := s.getRunningServer(ctx, rr.serverID)
		if err != nil {
			continue
		}
		value, err := s.alertMetricValue(ctx, record, rr.metric)
		if err != nil {
			slog.Warn("alertas: falha lendo métrica", "rule_id", rr.id, "metric", rr.metric, "error", err)
			continue
		}

		breached := value >= rr.threshold
		if rr.metric == "deadlocks" {
			delta := value - float64(rr.lastDeadlockCount)
			breached = delta >= rr.threshold
			s.repo.pool.Exec(ctx, `UPDATE alert_rules SET last_deadlock_count = $1 WHERE id = $2`, int64(value), rr.id)
			value = delta
		}

		s.repo.pool.Exec(ctx, `UPDATE alert_rules SET last_value = $1 WHERE id = $2`, value, rr.id)

		if !breached {
			continue
		}

		payload := webhookPayload{
			ServerID:    rr.serverID,
			ServerName:  rr.serverName,
			Metric:      rr.metric,
			Description: allowedAlertMetrics[rr.metric],
			Value:       value,
			Threshold:   rr.threshold,
			TriggeredAt: time.Now(),
		}

		var sendErr error
		if rr.channelID != "" {
			channel, err := s.getNotificationChannel(ctx, rr.channelID)
			if err != nil {
				slog.Warn("alertas: canal de notificação não encontrado", "rule_id", rr.id, "channel_id", rr.channelID, "error", err)
			} else {
				sendErr = s.sendNotification(ctx, channel, payload)
			}
		} else if rr.webhookURL != "" {
			sendErr = s.sendNotification(ctx, &NotificationChannel{Kind: ChannelWebhook, WebhookURL: rr.webhookURL}, payload)
		}
		if sendErr != nil {
			slog.Warn("alertas: falha enviando notificação", "rule_id", rr.id, "error", sendErr)
		}
		s.repo.pool.Exec(ctx, `UPDATE alert_rules SET last_triggered_at = now() WHERE id = $1`, rr.id)
	}
}
