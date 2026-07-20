package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleViewer Role = "viewer"
)

func (r Role) valid() bool { return r == RoleAdmin || r == RoleViewer }

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, username, role, created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listando usuários: %w", err)
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("lendo usuário: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Service) CreateUser(ctx context.Context, username, password string, role Role) (*User, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("usuário e senha são obrigatórios")
	}
	if !role.valid() {
		return nil, fmt.Errorf("papel deve ser 'admin' ou 'viewer'")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("gerando hash da senha: %w", err)
	}

	var u User
	err = s.pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, $3)
		RETURNING id::text, username, role, created_at
	`, username, string(hash), role).Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("criando usuário (nome já em uso?): %w", err)
	}
	return &u, nil
}

// DeleteUser recusa apagar a própria conta (evita se trancar fora sem
// querer) e recusa apagar o último admin restante (sem isso a plataforma
// fica sem ninguém que consiga gerenciar usuário nenhum).
func (s *Service) DeleteUser(ctx context.Context, requestingUserID, targetUserID string) error {
	if requestingUserID == targetUserID {
		return fmt.Errorf("não é possível excluir a própria conta")
	}

	var targetRole Role
	err := s.pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, targetUserID).Scan(&targetRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("usuário não encontrado")
		}
		return fmt.Errorf("lendo usuário: %w", err)
	}

	if targetRole == RoleAdmin {
		var adminCount int
		if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'admin'`).Scan(&adminCount); err != nil {
			return fmt.Errorf("contando admins: %w", err)
		}
		if adminCount <= 1 {
			return fmt.Errorf("não é possível excluir o último admin")
		}
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, targetUserID); err != nil {
		return fmt.Errorf("excluindo usuário: %w", err)
	}
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, userID, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("senha nova é obrigatória")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("gerando hash da senha: %w", err)
	}
	tag, err := s.pool.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, userID, string(hash))
	if err != nil {
		return fmt.Errorf("trocando senha: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("usuário não encontrado")
	}
	return nil
}
