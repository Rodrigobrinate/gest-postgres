// Package server (db_objects.go): Views, Materialized Views, Sequences e
// Types/Domains — os objetos de banco mais comuns depois de tabelas, que não
// justificam um arquivo próprio cada um dado o tamanho parecido do CRUD.
package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ---------- Views ----------

type ViewInfo struct {
	Schema     string `json:"schema"`
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

func (s *Service) ListViews(ctx context.Context, id, database string) ([]ViewInfo, error) {
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
		SELECT schemaname, viewname, definition
		FROM pg_views
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY schemaname, viewname
	`)
	if err != nil {
		return nil, fmt.Errorf("listando views: %w", err)
	}
	defer rows.Close()

	views := make([]ViewInfo, 0)
	for rows.Next() {
		var v ViewInfo
		if err := rows.Scan(&v.Schema, &v.Name, &v.Definition); err != nil {
			return nil, fmt.Errorf("lendo view: %w", err)
		}
		views = append(views, v)
	}
	return views, rows.Err()
}

func (s *Service) CreateView(ctx context.Context, id, database, schema, name, query string) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	sql := fmt.Sprintf("CREATE VIEW %s AS %s", pgx.Identifier{schema, name}.Sanitize(), query)
	return s.execDDL(ctx, id, database, sql)
}

func (s *Service) DropView(ctx context.Context, id, database, schema, name string) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	sql := "DROP VIEW " + pgx.Identifier{schema, name}.Sanitize()
	return s.execDDL(ctx, id, database, sql)
}

// ---------- Materialized Views ----------

type MaterializedViewInfo struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	Populated   bool   `json:"populated"`
	SizeBytes   int64  `json:"size_bytes"`
	Definition  string `json:"definition"`
}

func (s *Service) ListMaterializedViews(ctx context.Context, id, database string) ([]MaterializedViewInfo, error) {
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
			m.schemaname,
			m.matviewname,
			m.ispopulated,
			pg_total_relation_size(quote_ident(m.schemaname) || '.' || quote_ident(m.matviewname)),
			m.definition
		FROM pg_matviews m
		ORDER BY m.schemaname, m.matviewname
	`)
	if err != nil {
		return nil, fmt.Errorf("listando materialized views: %w", err)
	}
	defer rows.Close()

	views := make([]MaterializedViewInfo, 0)
	for rows.Next() {
		var v MaterializedViewInfo
		if err := rows.Scan(&v.Schema, &v.Name, &v.Populated, &v.SizeBytes, &v.Definition); err != nil {
			return nil, fmt.Errorf("lendo materialized view: %w", err)
		}
		views = append(views, v)
	}
	return views, rows.Err()
}

func (s *Service) CreateMaterializedView(ctx context.Context, id, database, schema, name, query string) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	sql := fmt.Sprintf("CREATE MATERIALIZED VIEW %s AS %s", pgx.Identifier{schema, name}.Sanitize(), query)
	return s.execDDL(ctx, id, database, sql)
}

func (s *Service) RefreshMaterializedView(ctx context.Context, id, database, schema, name string, concurrently bool) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	sql := "REFRESH MATERIALIZED VIEW "
	if concurrently {
		sql += "CONCURRENTLY "
	}
	sql += pgx.Identifier{schema, name}.Sanitize()
	return s.execDDL(ctx, id, database, sql)
}

func (s *Service) DropMaterializedView(ctx context.Context, id, database, schema, name string) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	sql := "DROP MATERIALIZED VIEW " + pgx.Identifier{schema, name}.Sanitize()
	return s.execDDL(ctx, id, database, sql)
}

// ---------- Sequences ----------

type SequenceInfo struct {
	Schema    string `json:"schema"`
	Name      string `json:"name"`
	LastValue *int64 `json:"last_value"`
	Increment int64  `json:"increment"`
	MinValue  int64  `json:"min_value"`
	MaxValue  int64  `json:"max_value"`
	CacheSize int64  `json:"cache_size"`
	Cycle     bool   `json:"cycle"`
}

func (s *Service) ListSequences(ctx context.Context, id, database string) ([]SequenceInfo, error) {
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
		SELECT schemaname, sequencename, last_value, increment_by, min_value, max_value, cache_size, cycle
		FROM pg_sequences
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY schemaname, sequencename
	`)
	if err != nil {
		return nil, fmt.Errorf("listando sequences: %w", err)
	}
	defer rows.Close()

	seqs := make([]SequenceInfo, 0)
	for rows.Next() {
		var sq SequenceInfo
		if err := rows.Scan(&sq.Schema, &sq.Name, &sq.LastValue, &sq.Increment, &sq.MinValue, &sq.MaxValue, &sq.CacheSize, &sq.Cycle); err != nil {
			return nil, fmt.Errorf("lendo sequence: %w", err)
		}
		seqs = append(seqs, sq)
	}
	return seqs, rows.Err()
}

type CreateSequenceInput struct {
	Schema    string `json:"schema"`
	Name      string `json:"name"`
	Increment int64  `json:"increment"`
	StartWith int64  `json:"start_with"`
	Cycle     bool   `json:"cycle"`
}

func (s *Service) CreateSequence(ctx context.Context, id, database string, in CreateSequenceInput) error {
	if in.Schema == "" {
		in.Schema = "public"
	}
	if !identRegex.MatchString(in.Schema) || !identRegex.MatchString(in.Name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	if in.Increment == 0 {
		in.Increment = 1
	}
	if in.StartWith == 0 {
		in.StartWith = 1
	}

	sql := fmt.Sprintf("CREATE SEQUENCE %s INCREMENT BY %d START WITH %d",
		pgx.Identifier{in.Schema, in.Name}.Sanitize(), in.Increment, in.StartWith)
	if in.Cycle {
		sql += " CYCLE"
	}
	return s.execDDL(ctx, id, database, sql)
}

func (s *Service) DropSequence(ctx context.Context, id, database, schema, name string) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	sql := "DROP SEQUENCE " + pgx.Identifier{schema, name}.Sanitize()
	return s.execDDL(ctx, id, database, sql)
}

// ---------- Types / Domains ----------

type TypeInfo struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Kind   string `json:"kind"` // "enum" | "domain" | "composite"
	Detail string `json:"detail"`
}

func (s *Service) ListTypes(ctx context.Context, id, database string) ([]TypeInfo, error) {
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
			t.typname,
			CASE
				WHEN t.typtype = 'e' THEN 'enum'
				WHEN t.typtype = 'd' THEN 'domain'
				WHEN t.typtype = 'c' THEN 'composite'
				ELSE t.typtype::text
			END,
			CASE
				WHEN t.typtype = 'e' THEN (
					SELECT string_agg(e.enumlabel, ', ' ORDER BY e.enumsortorder)
					FROM pg_enum e WHERE e.enumtypid = t.oid
				)
				WHEN t.typtype = 'd' THEN format_type(t.typbasetype, t.typtypmod)
				ELSE ''
			END
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE t.typtype IN ('e', 'd', 'c')
			AND n.nspname NOT IN ('pg_catalog', 'information_schema')
			AND NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = t.typrelid AND c.relkind != 'c')
		ORDER BY n.nspname, t.typname
	`)
	if err != nil {
		return nil, fmt.Errorf("listando types: %w", err)
	}
	defer rows.Close()

	types := make([]TypeInfo, 0)
	for rows.Next() {
		var t TypeInfo
		var detail *string
		if err := rows.Scan(&t.Schema, &t.Name, &t.Kind, &detail); err != nil {
			return nil, fmt.Errorf("lendo type: %w", err)
		}
		if detail != nil {
			t.Detail = *detail
		}
		types = append(types, t)
	}
	return types, rows.Err()
}

func (s *Service) CreateEnumType(ctx context.Context, id, database, schema, name string, values []string) error {
	if schema == "" {
		schema = "public"
	}
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	if len(values) == 0 {
		return fmt.Errorf("%w: enum precisa de pelo menos um valor", ErrValidation)
	}

	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = sqlQuoteLiteral(v)
	}

	sql := fmt.Sprintf("CREATE TYPE %s AS ENUM (%s)",
		pgx.Identifier{schema, name}.Sanitize(), strings.Join(quoted, ", "))
	return s.execDDL(ctx, id, database, sql)
}

func (s *Service) CreateDomain(ctx context.Context, id, database, schema, name, baseType, checkExpr string) error {
	if schema == "" {
		schema = "public"
	}
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	if !allowedColumnTypes[strings.ToLower(baseType)] {
		return fmt.Errorf("%w: tipo base %q não suportado", ErrValidation, baseType)
	}

	sql := fmt.Sprintf("CREATE DOMAIN %s AS %s", pgx.Identifier{schema, name}.Sanitize(), baseType)
	if checkExpr != "" {
		if strings.Contains(checkExpr, ";") {
			return fmt.Errorf("%w: constraint não pode conter ';'", ErrValidation)
		}
		sql += fmt.Sprintf(" CHECK (%s)", checkExpr)
	}
	return s.execDDL(ctx, id, database, sql)
}

func (s *Service) DropType(ctx context.Context, id, database, schema, name string) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	sql := "DROP TYPE " + pgx.Identifier{schema, name}.Sanitize()
	return s.execDDL(ctx, id, database, sql)
}
