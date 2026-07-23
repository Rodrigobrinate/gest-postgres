package server

import (
	"context"
	"fmt"
	"strings"
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
	Schema        string `json:"schema"`
	Name          string `json:"name"`
	SizeBytes     int64  `json:"size_bytes"`
	EstimatedRows int64  `json:"estimated_rows"`
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

type DatabaseSize struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
}

func (s *Service) DatabaseSizes(ctx context.Context, id string) ([]DatabaseSize, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, "")
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT datname, pg_database_size(datname)
		FROM pg_database
		WHERE datistemplate = false
		ORDER BY pg_database_size(datname) DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("lendo tamanho dos bancos: %w", err)
	}
	defer rows.Close()

	sizes := make([]DatabaseSize, 0)
	for rows.Next() {
		var d DatabaseSize
		if err := rows.Scan(&d.Name, &d.SizeBytes); err != nil {
			return nil, fmt.Errorf("lendo tamanho: %w", err)
		}
		sizes = append(sizes, d)
	}
	return sizes, rows.Err()
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

// RowFilter é um filtro simples de coluna pro grid de dados da tabela (tipo
// Prisma Studio) — WHERE column op value. Column vira identificador
// sanitizado (pgx.Identifier.Sanitize, mesma defesa de CreateTable/DropTable);
// op é resolvido contra allowedFilterOps (whitelist fixa, nunca texto do
// usuário vira SQL); value sempre vira parâmetro ligado ($n), nunca
// concatenado — mesmo pro contains, que só monta o padrão ILIKE em Go antes
// de virar parâmetro.
type RowFilter struct {
	Column string `json:"column"`
	Op     string `json:"op"`
	Value  string `json:"value"`
}

// allowedFilterOps mapeia operador (vindo do frontend) pro operador SQL —
// nunca interpola o texto do operador vindo do usuário direto na query.
var allowedFilterOps = map[string]string{
	"eq":          "=",
	"neq":         "<>",
	"gt":          ">",
	"gte":         ">=",
	"lt":          "<",
	"lte":         "<=",
	"contains":    "ILIKE",
	"is_null":     "IS NULL",
	"is_not_null": "IS NOT NULL",
}

func buildRowFilterSQL(filters []RowFilter) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	var clauses []string
	var args []any
	for _, f := range filters {
		if !identRegex.MatchString(f.Column) {
			return "", nil, fmt.Errorf("%w: coluna de filtro %q inválida", ErrValidation, f.Column)
		}
		opSQL, ok := allowedFilterOps[f.Op]
		if !ok {
			return "", nil, fmt.Errorf("%w: operador de filtro %q inválido", ErrValidation, f.Op)
		}
		colIdent := pgx.Identifier{f.Column}.Sanitize()
		switch f.Op {
		case "is_null", "is_not_null":
			clauses = append(clauses, fmt.Sprintf("%s %s", colIdent, opSQL))
		case "contains":
			args = append(args, "%"+f.Value+"%")
			// ::text — ILIKE exige texto dos dois lados; único operador que
			// precisa de cast explícito na coluna (os outros deixam o
			// parâmetro sem tipo, Postgres infere pelo tipo real da coluna,
			// o que preserva comparação numérica/data em vez de virar
			// comparação lexicográfica de texto).
			clauses = append(clauses, fmt.Sprintf("%s::text %s $%d", colIdent, opSQL, len(args)))
		default:
			args = append(args, f.Value)
			clauses = append(clauses, fmt.Sprintf("%s %s $%d", colIdent, opSQL, len(args)))
		}
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

// TableRows retorna uma página de dados da tabela, mais o total de linhas —
// com ordenação e filtro simples por coluna (grid tipo Prisma Studio, pedido
// explícito do usuário, 2026-07-22). Total é count(*) exato (com o mesmo
// WHERE do filtro) — aceitável no MVP, mas fica lento em tabelas gigantes;
// trocar por estimativa via pg_class.reltuples se virar problema.
func (s *Service) TableRows(
	ctx context.Context, id, database, schema, table string,
	limit, offset int, sortColumn string, sortDesc bool, filters []RowFilter,
) (*QueryResult, int64, error) {
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

	whereClause, args, err := buildRowFilterSQL(filters)
	if err != nil {
		return nil, 0, err
	}

	var orderClause string
	if sortColumn != "" {
		if !identRegex.MatchString(sortColumn) {
			return nil, 0, fmt.Errorf("%w: coluna de ordenação %q inválida", ErrValidation, sortColumn)
		}
		dir := "ASC"
		if sortDesc {
			dir = "DESC"
		}
		orderClause = fmt.Sprintf(" ORDER BY %s %s", pgx.Identifier{sortColumn}.Sanitize(), dir)
	}

	var total int64
	countSQL := "SELECT count(*) FROM " + ident + whereClause
	if err := conn.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	dataSQL := fmt.Sprintf("SELECT * FROM %s%s%s LIMIT %d OFFSET %d", ident, whereClause, orderClause, limit, offset)
	result, err := runAndCollect(ctx, conn, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

func runAndCollect(ctx context.Context, conn *pgx.Conn, sql string, args ...any) (*QueryResult, error) {
	start := time.Now()

	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		// Erro de sintaxe, tabela inexistente, permissão negada etc — é o
		// usuário que escreveu a query errada, não um bug da plataforma. Marca
		// como ErrValidation pra virar um 4xx com a mensagem real do Postgres em
		// vez de sumir num "erro interno" genérico.
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
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
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
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
	// pgx decodifica coluna uuid pra [16]byte (array de tamanho fixo, não
	// slice — não bate no case "[]byte" acima nem implementa fmt.Stringer),
	// então sem esse case o encoding/json despeja os 16 bytes crus como
	// array de números (bug visto ao vivo: coluna id virando
	// "[176,70,128,...]" em toda tabela com PK uuid).
	case [16]byte:
		return formatUUID(val)
	case fmt.Stringer:
		return val.String()
	default:
		return v
	}
}

// formatUUID formata 16 bytes crus no formato canônico 8-4-4-4-12.
func formatUUID(b [16]byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
