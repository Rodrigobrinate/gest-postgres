package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type FunctionInfo struct {
	Schema       string `json:"schema"`
	Name         string `json:"name"`
	Arguments    string `json:"arguments"`
	ReturnType   string `json:"return_type"`
	Kind         string `json:"kind"` // "function" | "procedure"
	Language     string `json:"language"`
	Definition   string `json:"definition"`
	IdentityArgs string `json:"identity_args"` // usado só pra montar o DROP (permite overload)
}

func (s *Service) ListFunctions(ctx context.Context, id, database string) ([]FunctionInfo, error) {
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
		SELECT
			n.nspname,
			p.proname,
			pg_get_function_arguments(p.oid),
			CASE WHEN p.prokind = 'p' THEN '' ELSE pg_get_function_result(p.oid) END,
			CASE WHEN p.prokind = 'p' THEN 'procedure' ELSE 'function' END,
			l.lanname,
			pg_get_functiondef(p.oid),
			pg_get_function_identity_arguments(p.oid)
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		JOIN pg_language l ON l.oid = p.prolang
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
			AND p.prokind IN ('f', 'p')
		ORDER BY n.nspname, p.proname
	`)
	if err != nil {
		return nil, fmt.Errorf("listando functions: %w", err)
	}
	defer rows.Close()

	fns := make([]FunctionInfo, 0)
	for rows.Next() {
		var f FunctionInfo
		if err := rows.Scan(&f.Schema, &f.Name, &f.Arguments, &f.ReturnType, &f.Kind, &f.Language, &f.Definition, &f.IdentityArgs); err != nil {
			return nil, fmt.Errorf("lendo function: %w", err)
		}
		fns = append(fns, f)
	}
	return fns, rows.Err()
}

// CreateFunction executa o statement CREATE FUNCTION/PROCEDURE cru que o
// usuário escreveu — parâmetros, corpo e linguagem variam demais pra valer a
// pena modelar como formulário estruturado. Mesma fronteira de confiança do
// editor SQL (já dá pra rodar isso por lá também).
func (s *Service) CreateFunction(ctx context.Context, id, database, sql string) error {
	trimmed := strings.TrimSpace(strings.ToUpper(sql))
	if !strings.HasPrefix(trimmed, "CREATE FUNCTION") &&
		!strings.HasPrefix(trimmed, "CREATE OR REPLACE FUNCTION") &&
		!strings.HasPrefix(trimmed, "CREATE PROCEDURE") &&
		!strings.HasPrefix(trimmed, "CREATE OR REPLACE PROCEDURE") {
		return fmt.Errorf("%w: precisa começar com CREATE [OR REPLACE] FUNCTION/PROCEDURE", ErrValidation)
	}
	return s.execDDL(ctx, id, database, sql)
}

func (s *Service) DropFunction(ctx context.Context, id, database, schema, name, identityArgs string) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	if strings.Contains(identityArgs, ";") {
		return fmt.Errorf("%w: assinatura inválida", ErrValidation)
	}
	sql := fmt.Sprintf("DROP FUNCTION %s(%s)", pgx.Identifier{schema, name}.Sanitize(), identityArgs)
	return s.execDDL(ctx, id, database, sql)
}
