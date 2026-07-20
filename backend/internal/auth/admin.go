package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
)

const defaultAdminUsername = "admin"

// SeedAdminIfMissing garante que existe pelo menos 1 usuário (sempre admin,
// o primeiro). Idempotente — roda em todo boot do backend, só faz algo na
// primeira vez. Se passwordFromEnv vier vazia (ADMIN_PASSWORD não
// configurada em .env), gera uma senha aleatória e loga ela uma vez só —
// não sobe sem login nenhum.
func (s *Service) SeedAdminIfMissing(ctx context.Context, passwordFromEnv string) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return fmt.Errorf("checando usuários existentes: %w", err)
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

	if _, err := s.CreateUser(ctx, defaultAdminUsername, password, RoleAdmin); err != nil {
		return fmt.Errorf("criando admin inicial: %w", err)
	}
	return nil
}
