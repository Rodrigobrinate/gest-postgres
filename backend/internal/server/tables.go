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
