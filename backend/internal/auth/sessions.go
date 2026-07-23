package auth

import (
	"context"
	"fmt"
	"time"
)

// onlineWindow decide o badge "online" na tela de Gestão de sessões —
// last_seen_at é estendido a CADA requisição autenticada (ver
// ValidateSession), então uma aba aberta de verdade bate esse relógio o
// tempo todo; sessão que parou de ser usada (aba fechada) sai de "online"
// rápido, mesmo sem deslogar.
const onlineWindow = 2 * time.Minute

type SessionInfo struct {
	ID         string     `json:"id"`
	Username   string     `json:"username"`
	Role       Role       `json:"role"`
	IPAddress  string     `json:"ip_address"`
	UserAgent  string     `json:"user_agent"`
	CreatedAt  time.Time  `json:"created_at"`
	LastSeenAt time.Time  `json:"last_seen_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	Online     bool       `json:"online"`
	Current    bool       `json:"current"` // marcado no handler (é quem faz a própria requisição), não aqui
}

// ListActiveSessions lista sessão viva (não revogada, não expirada) de
// QUALQUER usuário — quem tá logado agora, ordenado pela mais recentemente
// ativa primeiro.
func (s *Service) ListActiveSessions(ctx context.Context) ([]SessionInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.id::text, u.username, u.role, COALESCE(s.ip_address, ''), COALESCE(s.user_agent, ''),
		       s.created_at, s.last_seen_at, s.expires_at
		FROM admin_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.revoked_at IS NULL AND s.expires_at > now()
		ORDER BY s.last_seen_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listando sessões ativas: %w", err)
	}
	defer rows.Close()

	out := []SessionInfo{}
	for rows.Next() {
		var si SessionInfo
		if err := rows.Scan(&si.ID, &si.Username, &si.Role, &si.IPAddress, &si.UserAgent,
			&si.CreatedAt, &si.LastSeenAt, &si.ExpiresAt); err != nil {
			return nil, fmt.Errorf("lendo sessão: %w", err)
		}
		si.Online = time.Since(si.LastSeenAt) < onlineWindow
		out = append(out, si)
	}
	return out, rows.Err()
}

// ListSessionHistory lista sessão de QUALQUER estado (ativa, revogada,
// expirada) — a mais recente criada primeiro. É o "log de sessão" da tela:
// quem logou, de onde, quando, e se/quando saiu.
func (s *Service) ListSessionHistory(ctx context.Context, limit int) ([]SessionInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.id::text, u.username, u.role, COALESCE(s.ip_address, ''), COALESCE(s.user_agent, ''),
		       s.created_at, s.last_seen_at, s.expires_at, s.revoked_at
		FROM admin_sessions s
		JOIN users u ON u.id = s.user_id
		ORDER BY s.created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("listando histórico de sessão: %w", err)
	}
	defer rows.Close()

	out := []SessionInfo{}
	for rows.Next() {
		var si SessionInfo
		if err := rows.Scan(&si.ID, &si.Username, &si.Role, &si.IPAddress, &si.UserAgent,
			&si.CreatedAt, &si.LastSeenAt, &si.ExpiresAt, &si.RevokedAt); err != nil {
			return nil, fmt.Errorf("lendo sessão: %w", err)
		}
		si.Online = si.RevokedAt == nil && si.ExpiresAt.After(time.Now()) && time.Since(si.LastSeenAt) < onlineWindow
		out = append(out, si)
	}
	return out, rows.Err()
}

// RevokeSession derruba uma sessão na hora (botão "encerrar" na tela de
// Gestão de sessões) — qualquer admin pode encerrar qualquer sessão,
// inclusive a própria ou a de outro admin (não tem proteção especial tipo
// "não pode excluir o último admin" aqui, porque isso não apaga a CONTA,
// só o acesso de UM dispositivo/navegador — a pessoa loga de novo se for
// ela mesma).
func (s *Service) RevokeSession(ctx context.Context, sessionID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE admin_sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL
	`, sessionID)
	if err != nil {
		return fmt.Errorf("encerrando sessão: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("sessão não encontrada ou já encerrada")
	}
	return nil
}
