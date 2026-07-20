package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/bcrypt"
)

const defaultAdminUsername = "admin"

// SeedAdminIfMissing garante que existe 1 linha em admin_user. Idempotente —
// roda em todo boot do backend, só faz algo na primeira vez. Se
// passwordFromEnv vier vazia (ADMIN_PASSWORD não configurada em .env), gera
// uma senha aleatória e loga ela uma vez só — não fica sem login nenhum.
func (s *Service) SeedAdminIfMissing(ctx context.Context, passwordFromEnv string) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM admin_user WHERE id = 1)`).Scan(&exists); err != nil {
		return fmt.Errorf("checando admin existente: %w", err)
	}
	if exists {
		return nil
	}

	password := passwordFromEnv
	if password == "" {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			return fmt.Errorf("gerando senha de admin aleatória: %w", err)
		}
		password = hex.EncodeToString(buf)
		slog.Warn("ADMIN_PASSWORD não configurada — gerei uma senha aleatória, guarde agora, não aparece de novo",
			"username", defaultAdminUsername, "password", password)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("gerando hash da senha de admin: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO admin_user (id, username, password_hash)
		VALUES (1, $1, $2)
	`, defaultAdminUsername, string(hash))
	if err != nil {
		return fmt.Errorf("criando admin: %w", err)
	}
	return nil
}
