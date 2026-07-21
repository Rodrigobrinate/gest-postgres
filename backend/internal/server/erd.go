package server

import (
	"context"
	"fmt"
)

type ERDColumn struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	PrimaryKey bool   `json:"primary_key"`
}

type ERDTable struct {
	Schema  string      `json:"schema"`
	Name    string      `json:"name"`
	Columns []ERDColumn `json:"columns"`
}

type ERDRelationship struct {
	ConstraintName string `json:"constraint_name"`
	FromSchema     string `json:"from_schema"`
	FromTable      string `json:"from_table"`
	FromColumn     string `json:"from_column"`
	ToSchema       string `json:"to_schema"`
	ToTable        string `json:"to_table"`
	ToColumn       string `json:"to_column"`
}

type ERD struct {
	Tables        []ERDTable        `json:"tables"`
	Relationships []ERDRelationship `json:"relationships"`
}

// GetERD introspecciona o schema inteiro (todo schema não-sistema) de um
// banco — tabelas, colunas (com tipo/nullable/PK) e foreign keys — pro
// diagrama entidade-relacionamento da aba ERD. Só lê catálogo
// (information_schema), nunca dado de tabela do usuário.
func (s *Service) GetERD(ctx context.Context, id, database string) (*ERD, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	tableIndex := make(map[string]*ERDTable)
	var order []string

	colRows, err := conn.Query(ctx, `
		SELECT table_schema, table_name, column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name, ordinal_position
	`)
	if err != nil {
		return nil, fmt.Errorf("listando colunas: %w", err)
	}
	for colRows.Next() {
		var schema, table, col, dataType, nullable string
		if err := colRows.Scan(&schema, &table, &col, &dataType, &nullable); err != nil {
			colRows.Close()
			return nil, fmt.Errorf("lendo coluna: %w", err)
		}
		key := schema + "." + table
		t, ok := tableIndex[key]
		if !ok {
			t = &ERDTable{Schema: schema, Name: table}
			tableIndex[key] = t
			order = append(order, key)
		}
		t.Columns = append(t.Columns, ERDColumn{Name: col, Type: dataType, Nullable: nullable == "YES"})
	}
	if err := colRows.Err(); err != nil {
		colRows.Close()
		return nil, fmt.Errorf("listando colunas: %w", err)
	}
	colRows.Close()

	pkRows, err := conn.Query(ctx, `
		SELECT tc.table_schema, tc.table_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
			AND tc.table_schema NOT IN ('pg_catalog', 'information_schema')
	`)
	if err != nil {
		return nil, fmt.Errorf("listando chaves primárias: %w", err)
	}
	for pkRows.Next() {
		var schema, table, col string
		if err := pkRows.Scan(&schema, &table, &col); err != nil {
			pkRows.Close()
			return nil, fmt.Errorf("lendo chave primária: %w", err)
		}
		if t, ok := tableIndex[schema+"."+table]; ok {
			for i := range t.Columns {
				if t.Columns[i].Name == col {
					t.Columns[i].PrimaryKey = true
				}
			}
		}
	}
	if err := pkRows.Err(); err != nil {
		pkRows.Close()
		return nil, fmt.Errorf("listando chaves primárias: %w", err)
	}
	pkRows.Close()

	fkRows, err := conn.Query(ctx, `
		SELECT
			tc.constraint_name,
			tc.table_schema, tc.table_name, kcu.column_name,
			ccu.table_schema, ccu.table_name, ccu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema NOT IN ('pg_catalog', 'information_schema')
	`)
	if err != nil {
		return nil, fmt.Errorf("listando chaves estrangeiras: %w", err)
	}
	defer fkRows.Close()

	var rels []ERDRelationship
	for fkRows.Next() {
		var r ERDRelationship
		if err := fkRows.Scan(
			&r.ConstraintName,
			&r.FromSchema, &r.FromTable, &r.FromColumn,
			&r.ToSchema, &r.ToTable, &r.ToColumn,
		); err != nil {
			return nil, fmt.Errorf("lendo chave estrangeira: %w", err)
		}
		rels = append(rels, r)
	}
	if err := fkRows.Err(); err != nil {
		return nil, fmt.Errorf("listando chaves estrangeiras: %w", err)
	}

	tables := make([]ERDTable, 0, len(order))
	for _, key := range order {
		tables = append(tables, *tableIndex[key])
	}

	return &ERD{Tables: tables, Relationships: rels}, nil
}
