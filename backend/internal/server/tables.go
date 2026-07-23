package server

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

var identRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// allowedColumnTypes é whitelist de propósito — o tipo da coluna vira SQL cru
// no DDL (não dá pra parametrizar tipo de coluna), então só aceita o que a
// gente reconhece. Cobre o suficiente pro formulário "criar tabela" do MVP.
var allowedColumnTypes = map[string]bool{
	"text": true, "varchar": true, "char": true,
	"integer": true, "bigint": true, "smallint": true,
	"serial": true, "bigserial": true,
	"boolean":   true,
	"timestamp": true, "timestamptz": true, "date": true, "time": true,
	"numeric": true, "real": true, "double precision": true,
	"uuid": true, "jsonb": true, "json": true,
}

type ColumnDef struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	NotNull    bool   `json:"not_null"`
	PrimaryKey bool   `json:"primary_key"`
	Default    string `json:"default"`
}

type CreateTableInput struct {
	Schema  string      `json:"schema"`
	Name    string      `json:"name"`
	Columns []ColumnDef `json:"columns"`
}

func (s *Service) CreateTable(ctx context.Context, id, database string, in CreateTableInput) error {
	if in.Schema == "" {
		in.Schema = "public"
	}
	if !identRegex.MatchString(in.Schema) {
		return fmt.Errorf("%w: schema %q inválido", ErrValidation, in.Schema)
	}
	if !identRegex.MatchString(in.Name) {
		return fmt.Errorf("%w: nome de tabela %q inválido", ErrValidation, in.Name)
	}
	if len(in.Columns) == 0 {
		return fmt.Errorf("%w: tabela precisa de pelo menos uma coluna", ErrValidation)
	}

	var colDefs []string
	var pkCols []string
	for _, col := range in.Columns {
		if !identRegex.MatchString(col.Name) {
			return fmt.Errorf("%w: nome de coluna %q inválido", ErrValidation, col.Name)
		}
		if !allowedColumnTypes[strings.ToLower(col.Type)] {
			return fmt.Errorf("%w: tipo de coluna %q não suportado", ErrValidation, col.Type)
		}

		def := pgx.Identifier{col.Name}.Sanitize() + " " + col.Type
		if col.NotNull {
			def += " NOT NULL"
		}
		if col.Default != "" {
			// Default é expressão SQL livre (now(), gen_random_uuid(), 'x', 0...) —
			// mesma fronteira de confiança do editor SQL, mas barra ; pra não
			// virar um segundo statement escondido dentro do CREATE TABLE.
			if strings.Contains(col.Default, ";") {
				return fmt.Errorf("%w: default da coluna %q não pode conter ';'", ErrValidation, col.Name)
			}
			def += " DEFAULT " + col.Default
		}
		colDefs = append(colDefs, def)

		if col.PrimaryKey {
			pkCols = append(pkCols, pgx.Identifier{col.Name}.Sanitize())
		}
	}

	if len(pkCols) > 0 {
		colDefs = append(colDefs, "PRIMARY KEY ("+strings.Join(pkCols, ", ")+")")
	}

	tableIdent := pgx.Identifier{in.Schema, in.Name}.Sanitize()
	sql := fmt.Sprintf("CREATE TABLE %s (\n\t%s\n)", tableIdent, strings.Join(colDefs, ",\n\t"))

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

func (s *Service) DropTable(ctx context.Context, id, database, schema, name string) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	sql := "DROP TABLE " + pgx.Identifier{schema, name}.Sanitize()
	return s.execDDL(ctx, id, database, sql)
}

// ColumnMeta descreve uma coluna real da tabela (não o formulário de criação
// — ColumnDef acima é isso) — usado pelo grid de dados (tipo Prisma Studio)
// pra saber o que é editável e qual coluna forma a chave primária.
type ColumnMeta struct {
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	IsNullable   bool   `json:"is_nullable"`
	IsPrimaryKey bool   `json:"is_primary_key"`
}

// TableColumns lista as colunas reais de uma tabela (information_schema),
// com qual delas forma a chave primária — edição de célula/exclusão de linha
// só é possível quando a tabela tem PK (é o que identifica UMA linha exata
// no UPDATE/DELETE gerado).
func (s *Service) TableColumns(ctx context.Context, id, database, schema, table string) ([]ColumnMeta, error) {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(table) {
		return nil, fmt.Errorf("%w: schema/tabela inválido", ErrValidation)
	}
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	return s.tableColumnsOnConn(ctx, conn, schema, table)
}

func (s *Service) tableColumnsOnConn(ctx context.Context, conn *pgx.Conn, schema, table string) ([]ColumnMeta, error) {
	rows, err := conn.Query(ctx, `
		SELECT c.column_name, c.data_type, c.is_nullable = 'YES', COALESCE(pk.is_pk, false)
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT kcu.column_name, true AS is_pk
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
			  ON kcu.constraint_name = tc.constraint_name
			 AND kcu.table_schema = tc.table_schema
			 AND kcu.table_name = tc.table_name
			WHERE tc.constraint_type = 'PRIMARY KEY'
			  AND tc.table_schema = $1 AND tc.table_name = $2
		) pk ON pk.column_name = c.column_name
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position
	`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("listando colunas de %s.%s: %w", schema, table, err)
	}
	defer rows.Close()

	var cols []ColumnMeta
	for rows.Next() {
		var c ColumnMeta
		if err := rows.Scan(&c.Name, &c.DataType, &c.IsNullable, &c.IsPrimaryKey); err != nil {
			return nil, fmt.Errorf("lendo coluna: %w", err)
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func primaryKeyColumnNames(cols []ColumnMeta) []string {
	var names []string
	for _, c := range cols {
		if c.IsPrimaryKey {
			names = append(names, c.Name)
		}
	}
	return names
}

// buildPKWhere monta "col1 = $N+1 AND col2 = $N+2 ..." + os argumentos na
// mesma ordem, startIndex é quantos parâmetros $ já foram usados antes desse
// WHERE (ex: UPDATE usa $1 pro valor novo, PK começa em $2).
func buildPKWhere(pkCols []string, pk map[string]any, startIndex int) (string, []any, error) {
	if len(pk) != len(pkCols) {
		return "", nil, fmt.Errorf("%w: chave primária incompleta ou com coluna a mais", ErrValidation)
	}
	var clauses []string
	var args []any
	for _, col := range pkCols {
		v, ok := pk[col]
		if !ok {
			return "", nil, fmt.Errorf("%w: chave primária incompleta (faltando %q)", ErrValidation, col)
		}
		args = append(args, v)
		clauses = append(clauses, fmt.Sprintf("%s = $%d", pgx.Identifier{col}.Sanitize(), startIndex+len(args)))
	}
	return strings.Join(clauses, " AND "), args, nil
}

// UpdateTableRow edita UMA célula de UMA linha, identificada pela chave
// primária — é o "clicar na célula e editar" do grid tipo Prisma Studio.
// value chega como veio do JSON (string/número/bool/null); sem cast
// explícito no SQL, o Postgres infere o tipo do parâmetro pelo tipo real da
// coluna (mesmo raciocínio de buildRowFilterSQL).
func (s *Service) UpdateTableRow(
	ctx context.Context, id, database, schema, table, column string, value any, pk map[string]any,
) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(table) {
		return fmt.Errorf("%w: schema/tabela inválido", ErrValidation)
	}
	if !identRegex.MatchString(column) {
		return fmt.Errorf("%w: nome de coluna %q inválido", ErrValidation, column)
	}

	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	cols, err := s.tableColumnsOnConn(ctx, conn, schema, table)
	if err != nil {
		return err
	}
	pkCols := primaryKeyColumnNames(cols)
	if len(pkCols) == 0 {
		return fmt.Errorf("%w: tabela sem chave primária, não dá pra editar célula por aqui", ErrValidation)
	}
	whereClause, whereArgs, err := buildPKWhere(pkCols, pk, 1)
	if err != nil {
		return err
	}

	ident := pgx.Identifier{schema, table}.Sanitize()
	colIdent := pgx.Identifier{column}.Sanitize()
	sql := fmt.Sprintf("UPDATE %s SET %s = $1 WHERE %s", ident, colIdent, whereClause)
	args := append([]any{value}, whereArgs...)

	tag, err := conn.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("%w: linha não encontrada (dado pode ter mudado — recarregue a tabela)", ErrValidation)
	}
	return nil
}

// DeleteTableRow exclui UMA linha, identificada pela chave primária.
func (s *Service) DeleteTableRow(ctx context.Context, id, database, schema, table string, pk map[string]any) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(table) {
		return fmt.Errorf("%w: schema/tabela inválido", ErrValidation)
	}

	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	cols, err := s.tableColumnsOnConn(ctx, conn, schema, table)
	if err != nil {
		return err
	}
	pkCols := primaryKeyColumnNames(cols)
	if len(pkCols) == 0 {
		return fmt.Errorf("%w: tabela sem chave primária, não dá pra excluir linha por aqui", ErrValidation)
	}
	whereClause, args, err := buildPKWhere(pkCols, pk, 0)
	if err != nil {
		return err
	}

	ident := pgx.Identifier{schema, table}.Sanitize()
	sql := fmt.Sprintf("DELETE FROM %s WHERE %s", ident, whereClause)

	tag, err := conn.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("%w: linha não encontrada (dado pode ter mudado — recarregue a tabela)", ErrValidation)
	}
	return nil
}

// InsertTableRow cria uma linha nova — values só entra com as colunas que o
// formulário preencheu de propósito (coluna omitida usa o DEFAULT da
// tabela, ex: serial/gen_random_uuid()/now()). RETURNING * devolve a linha
// já com o que o Postgres gerou, pro grid mostrar sem precisar recarregar a
// página inteira.
func (s *Service) InsertTableRow(ctx context.Context, id, database, schema, table string, values map[string]any) (*QueryResult, error) {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(table) {
		return nil, fmt.Errorf("%w: schema/tabela inválido", ErrValidation)
	}

	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	ident := pgx.Identifier{schema, table}.Sanitize()

	var sql string
	var args []any
	if len(values) == 0 {
		sql = fmt.Sprintf("INSERT INTO %s DEFAULT VALUES RETURNING *", ident)
	} else {
		cols := make([]string, 0, len(values))
		placeholders := make([]string, 0, len(values))
		for col, v := range values {
			if !identRegex.MatchString(col) {
				return nil, fmt.Errorf("%w: nome de coluna %q inválido", ErrValidation, col)
			}
			args = append(args, v)
			cols = append(cols, pgx.Identifier{col}.Sanitize())
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		sql = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *", ident, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	}

	return runAndCollect(ctx, conn, sql, args...)
}
