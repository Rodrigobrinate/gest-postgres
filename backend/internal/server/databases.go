package server

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CreateDatabase roda fora de qualquer database específico (conecta no banco
// padrão do servidor) — CREATE DATABASE não pode rodar dentro de transação,
// e execDDL já usa Exec direto sem transação explícita, então serve aqui.
func (s *Service) CreateDatabase(ctx context.Context, id, name string) error {
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
