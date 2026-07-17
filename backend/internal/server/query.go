package server

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// QueryResult é o formato genérico de resposta do editor SQL e do visualizador
// de tabelas — colunas + linhas já serializáveis em JSON.
type QueryResult struct {
	Columns    []string `json:"columns"`
	Rows       [][]any  `json:"rows"`
	RowCount   int      `json:"row_count"`
	CommandTag string   `json:"command_tag,omitempty"`
	DurationMs int64    `json:"duration_ms"`
}

type TableInfo struct {
	Schema         string `json:"schema"`
	Name           string `json:"name"`
	SizeBytes      int64  `json:"size_bytes"`
	EstimatedRows  int64  `json:"estimated_rows"`
}

// RunQuery executa SQL arbitrário no banco indicado — é o editor SQL, então
// não restringe a comandos de leitura (o usuário já é o admin do servidor).
func (s *Service) RunQuery(ctx context.Context, id, database, sql string) (*QueryResult, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	return runAndCollect(ctx, conn, sql)
}

func (s *Service) ListDatabases(ctx context.Context, id string) ([]string, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, "")
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname`)
	if err != nil {
		return nil, fmt.Errorf("listando bancos: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("lendo nome do banco: %w", err)
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (s *Service) ListTables(ctx context.Context, id, database string) ([]TableInfo, error) {
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
			t.schemaname,
			t.tablename,
			pg_total_relation_size(quote_ident(t.schemaname) || '.' || quote_ident(t.tablename)) AS size_bytes,
			GREATEST(COALESCE(c.reltuples, 0), 0)::bigint AS estimated_rows
		FROM pg_tables t
		JOIN pg_namespace n ON n.nspname = t.schemaname
		JOIN pg_class c ON c.relname = t.tablename AND c.relnamespace = n.oid
		WHERE t.schemaname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY t.schemaname, t.tablename
	`)
	if err != nil {
		return nil, fmt.Errorf("listando tabelas: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var t TableInfo
		if err := rows.Scan(&t.Schema, &t.Name, &t.SizeBytes, &t.EstimatedRows); err != nil {
			return nil, fmt.Errorf("lendo tabela: %w", err)
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

// TableRows retorna uma página de dados da tabela, mais o total de linhas.
// Total é count(*) exato — aceitável no MVP, mas fica lento em tabelas
// gigantes; trocar por estimativa via pg_class.reltuples se virar problema.
func (s *Service) TableRows(ctx context.Context, id, database, schema, table string, limit, offset int) (*QueryResult, int64, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close(ctx)

	ident := pgx.Identifier{schema, table}.Sanitize()

	var total int64
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM "+ident).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contando linhas de %s: %w", ident, err)
	}

	result, err := runAndCollect(ctx, conn, fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", ident, limit, offset))
	if err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

func runAndCollect(ctx context.Context, conn *pgx.Conn, sql string) (*QueryResult, error) {
	start := time.Now()

	rows, err := conn.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("executando query: %w", err)
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	columns := make([]string, len(fields))
	for i, f := range fields {
		columns[i] = string(f.Name)
	}

	resultRows := make([][]any, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("lendo linha: %w", err)
		}
		row := make([]any, len(values))
		for i, v := range values {
			row[i] = jsonSafeValue(v)
		}
		resultRows = append(resultRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &QueryResult{
		Columns:    columns,
		Rows:       resultRows,
		RowCount:   len(resultRows),
		CommandTag: rows.CommandTag().String(),
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// jsonSafeValue converte valores decodificados pelo pgx (numeric, uuid, inet,
// bytea, etc) pra algo que encoding/json serializa de forma legível — em vez
// de despejar a struct interna do pgtype crua.
func jsonSafeValue(v any) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	case fmt.Stringer:
		return val.String()
	default:
		return v
	}
}
