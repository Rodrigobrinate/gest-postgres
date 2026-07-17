package server

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

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
	return s.execExtensionDDL(ctx, id, database, "CREATE EXTENSION IF NOT EXISTS "+pgx.Identifier{name}.Sanitize())
}

func (s *Service) DisableExtension(ctx context.Context, id, database, name string) error {
	return s.execExtensionDDL(ctx, id, database, "DROP EXTENSION IF EXISTS "+pgx.Identifier{name}.Sanitize())
}

func (s *Service) execExtensionDDL(ctx context.Context, id, database, sql string) error {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return nil
}
