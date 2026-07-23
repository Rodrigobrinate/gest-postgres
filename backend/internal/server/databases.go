package server

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CreateDatabase cria um banco novo com uma role isolada dona dele (mesmo
// mecanismo do "Criar banco de teste", generalizado por pedido explícito do
// usuário, 2026-07-23 — antes só criava o banco cru, dono do superuser).
// Devolve usuário/senha pra UI montar a connection string na hora; a senha
// não fica guardada na plataforma.
func (s *Service) CreateDatabase(ctx context.Context, id, name string) (*DatabaseCreationResult, error) {
	if !identRegex.MatchString(name) {
		return nil, fmt.Errorf("%w: nome de banco inválido", ErrValidation)
	}
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}

	password, err := s.provisionIsolatedDatabase(ctx, record, name)
	if err != nil {
		return nil, err
	}
	return &DatabaseCreationResult{Database: name, Username: name, Password: password}, nil
}

// createPlainDatabase cria um banco cru, dono do superuser — usado pelo
// fluxo de restore (backup.go), que precisa de um banco vazio pro
// pg_restore encher, não de uma role nova/isolada (quem conecta ali é
// sempre o superuser, via pg_restore). Continua sem transação de propósito
// — CREATE DATABASE não pode rodar dentro de uma.
func (s *Service) createPlainDatabase(ctx context.Context, id, name string) error {
	if !identRegex.MatchString(name) {
		return fmt.Errorf("%w: nome de banco inválido", ErrValidation)
	}
	sql := "CREATE DATABASE " + pgx.Identifier{name}.Sanitize()
	return s.execDDL(ctx, id, "", sql)
}

// DropDatabase usa WITH (FORCE) — Postgres 13+ — pra derrubar conexões
// abertas automaticamente em vez de falhar com "database is being accessed
// by other users", que seria o resultado mais comum (a própria plataforma
// costuma ter uma conexão de monitoramento aberta).
func (s *Service) DropDatabase(ctx context.Context, id, name string) error {
	if !identRegex.MatchString(name) {
		return fmt.Errorf("%w: nome de banco inválido", ErrValidation)
	}
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return err
	}
	if name == record.DatabaseName {
		return fmt.Errorf("%w: não dá pra excluir o banco principal do servidor", ErrValidation)
	}

	sql := "DROP DATABASE " + pgx.Identifier{name}.Sanitize() + " WITH (FORCE)"
	return s.execDDL(ctx, id, "", sql)
}
