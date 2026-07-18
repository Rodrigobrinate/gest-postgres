package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// extensionPreloadLib mapeia extensão -> lib que precisa estar em
// shared_preload_libraries pra ela funcionar de verdade (não é só instalar,
// precisa reiniciar o postmaster). A maioria das extensões não precisa disso
// — só as que registram hooks/background workers no processo do servidor.
var extensionPreloadLib = map[string]string{
	"pg_stat_statements": "pg_stat_statements",
	"pg_cron":            "pg_cron",
}

type Extension struct {
	Name             string `json:"name"`
	DefaultVersion   string `json:"default_version"`
	InstalledVersion string `json:"installed_version"` // vazio = não habilitada
	Comment          string `json:"comment"`
}

// ListExtensions junta o que a imagem suporta (pg_available_extensions) com o
// que já tá habilitado no banco (installed_version vem null quando não tá).
func (s *Service) ListExtensions(ctx context.Context, id, database string) ([]Extension, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT name, default_version, COALESCE(installed_version, ''), COALESCE(comment, '')
		FROM pg_available_extensions
		ORDER BY (installed_version IS NOT NULL) DESC, name
	`)
	if err != nil {
		return nil, fmt.Errorf("listando extensões: %w", err)
	}
	defer rows.Close()

	extensions := make([]Extension, 0)
	for rows.Next() {
		var e Extension
		if err := rows.Scan(&e.Name, &e.DefaultVersion, &e.InstalledVersion, &e.Comment); err != nil {
			return nil, fmt.Errorf("lendo extensão: %w", err)
		}
		extensions = append(extensions, e)
	}
	return extensions, rows.Err()
}

func (s *Service) EnableExtension(ctx context.Context, id, database, name string) error {
	lib, needsPreload := extensionPreloadLib[name]
	if !needsPreload {
		return s.execDDL(ctx, id, database, "CREATE EXTENSION IF NOT EXISTS "+pgx.Identifier{name}.Sanitize())
	}
	return s.enableExtensionWithPreload(ctx, id, database, name, lib)
}

// enablePreloadLib garante `lib` em shared_preload_libraries sem apagar
// outras libs já configuradas. Só grava via ALTER SYSTEM — quem chama decide
// quando reiniciar.
//
// A sintaxe do ALTER SYSTEM importa aqui de um jeito nada óbvio: passar o
// valor todo como UMA string literal com vírgula dentro (ex:
// `= 'pg_stat_statements,pg_cron'`) faz o Postgres persistir errado no
// postgresql.auto.conf — grava `= '"pg_stat_statements,pg_cron"'` (aspas
// duplas extras envolvendo tudo), e na subida seguinte ele tenta abrir UM
// arquivo de lib chamado literalmente "pg_stat_statements,pg_cron" e o
// servidor entra em crash loop (bug reproduzido e confirmado manualmente
// nessa sessão). A sintaxe multi-valor do ALTER SYSTEM (identificadores
// soltos separados por vírgula, sem string por fora) persiste correto.
func (s *Service) enablePreloadLib(ctx context.Context, record *Server, lib string) error {
	conn, err := s.connectTo(ctx, record, record.DatabaseName)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	var current string
	if err := conn.QueryRow(ctx, `SHOW shared_preload_libraries`).Scan(&current); err != nil {
		return fmt.Errorf("lendo shared_preload_libraries: %w", err)
	}
	if containsLib(current, lib) {
		return nil
	}

	libs := []string{lib}
	if current != "" {
		for _, l := range strings.Split(current, ",") {
			l = strings.TrimSpace(l)
			if l != "" {
				libs = append(libs, l)
			}
		}
	}
	for _, l := range libs {
		if !identRegex.MatchString(l) {
			return fmt.Errorf("%w: nome de lib inválido: %s", ErrValidation, l)
		}
	}

	sql := "ALTER SYSTEM SET shared_preload_libraries = " + strings.Join(libs, ", ")
	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("configurando shared_preload_libraries: %w", err)
	}
	return nil
}

// enableExtensionWithPreload cobre extensões tipo pg_cron/pg_stat_statements
// que só funcionam com a lib em shared_preload_libraries — isso exige
// restart do container, então o clique em "Habilitar" demora mais que uma
// extensão comum (e derruba conexões momentaneamente).
func (s *Service) enableExtensionWithPreload(ctx context.Context, id, database, name, lib string) error {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return err
	}

	if err := s.enablePreloadLib(ctx, record, lib); err != nil {
		return err
	}

	// pg_cron só roda numa única database do cluster (cron.database_name,
	// default 'postgres') — aponta pra onde o usuário tá habilitando.
	if name == "pg_cron" {
		conn, err := s.connectTo(ctx, record, record.DatabaseName)
		if err != nil {
			return err
		}
		_, err = conn.Exec(ctx, "ALTER SYSTEM SET cron.database_name = "+sqlQuoteLiteral(database))
		conn.Close(ctx)
		if err != nil {
			return fmt.Errorf("configurando cron.database_name: %w", err)
		}
	}

	if err := s.docker.RestartContainer(ctx, record.ContainerID); err != nil {
		return err
	}
	if err := s.docker.WaitHealthy(ctx, record.ContainerID, 60*time.Second); err != nil {
		return err
	}
	password, err := s.secretBox.Open(record.PasswordEncrypted)
	if err != nil {
		return err
	}
	if err := waitPostgresReady(ctx, record.ContainerName, record.Username, password, record.DatabaseName, 60*time.Second); err != nil {
		return err
	}

	return s.execDDL(ctx, id, database, "CREATE EXTENSION IF NOT EXISTS "+pgx.Identifier{name}.Sanitize())
}

func (s *Service) DisableExtension(ctx context.Context, id, database, name string) error {
	return s.execDDL(ctx, id, database, "DROP EXTENSION IF EXISTS "+pgx.Identifier{name}.Sanitize())
}
