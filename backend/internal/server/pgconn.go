package server

import (
	"context"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5"
)

// connectTo abre uma conexão direta com o Postgres gerenciado, via a rede
// Docker gestpg-managed (o backend está nela — ver docker-compose.yml), usando
// o nome do container como host. Nunca passa pelo host_port/localhost: mais
// rápido e não depende do container estar com a porta publicada.
func (s *Service) connectTo(ctx context.Context, record *Server, database string) (*pgx.Conn, error) {
	password, err := s.secretBox.Open(record.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decifrando senha do servidor: %w", err)
	}
	if database == "" {
		database = record.DatabaseName
	}

	// sslmode=prefer: tenta TLS, cai pra texto puro se o servidor não
	// oferecer — sem regressão pros nossos Postgres gerenciados (sem SSL
	// configurado), mas funciona contra um alvo adotado que exige SSL (ver
	// mesmo raciocínio em waitPostgresReady, service.go).
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:5432/%s?sslmode=prefer&connect_timeout=5",
		url.QueryEscape(record.Username), url.QueryEscape(password),
		record.ContainerName, url.QueryEscape(database),
	)

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("conectando ao servidor: %w", err)
	}
	return conn, nil
}

// getRunningServer busca o servidor e garante que dá pra conectar nele (senão
// os endpoints de query/activity/tabelas retornam um erro claro em vez de
// travar esperando um container parado/inexistente).
func (s *Service) getRunningServer(ctx context.Context, id string) (*Server, error) {
	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.Status != StatusRunning {
		return nil, fmt.Errorf("%w: servidor está %q, precisa estar rodando", ErrValidation, record.Status)
	}
	return record, nil
}
