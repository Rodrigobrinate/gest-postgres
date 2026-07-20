// Package auth implementa login de administrador único (usuário/senha) e
// sessões guardadas no banco de metadados — sobrevivem a restart do
// backend, ao contrário de um mapa em memória. Sem RBAC multi-nível
// (fora do MVP, ver CLAUDE.md).
package auth

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}
