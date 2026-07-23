package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type LoginAttempt struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Success   bool      `json:"success"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
}

// recordLoginAttempt é best-effort de propósito — uma falha gravando o log
// de auditoria nunca pode derrubar o fluxo de login em si (chamada sem
// checar erro de propósito, só loga).
func (s *Service) recordLoginAttempt(ctx context.Context, username string, success bool, ip, userAgent string) {
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO login_attempts (username, success, ip_address, user_agent)
		VALUES ($1, $2, $3, $4)
	`, username, success, ip, userAgent); err != nil {
		slog.Warn("falha registrando tentativa de login", "error", err)
	}
}

func (s *Service) ListLoginAttempts(ctx context.Context, limit int) ([]LoginAttempt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, username, success, ip_address, COALESCE(user_agent, ''), created_at
		FROM login_attempts
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("listando tentativas de login: %w", err)
	}
	defer rows.Close()

	out := []LoginAttempt{}
	for rows.Next() {
		var a LoginAttempt
		if err := rows.Scan(&a.ID, &a.Username, &a.Success, &a.IPAddress, &a.UserAgent, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("lendo tentativa de login: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// sessionLogRetention — mesmo espírito do retention de backup/rollup de
// métrica: log de segurança não fica pra sempre, mas também não some rápido
// demais a ponto de perder valor de auditoria.
const sessionLogRetention = 90 * 24 * time.Hour

// RunSessionRetentionSweep roda em background (chamado uma vez no main) —
// apaga só HISTÓRICO já encerrado (sessão revogada ou expirada, tentativa
// de login) mais velho que sessionLogRetention. Sessão ATIVA nunca é tocada
// aqui (não tem WHERE que bata nela).
func (s *Service) RunSessionRetentionSweep(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepSessionLogs(ctx)
		}
	}
}

func (s *Service) sweepSessionLogs(ctx context.Context) {
	cutoff := time.Now().Add(-sessionLogRetention)
	if _, err := s.pool.Exec(ctx, `
		DELETE FROM admin_sessions WHERE (revoked_at IS NOT NULL OR expires_at < now()) AND created_at < $1
	`, cutoff); err != nil {
		slog.Warn("sweep de sessão: falha apagando histórico de sessão", "error", err)
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM login_attempts WHERE created_at < $1`, cutoff); err != nil {
		slog.Warn("sweep de sessão: falha apagando tentativas de login", "error", err)
	}
}
