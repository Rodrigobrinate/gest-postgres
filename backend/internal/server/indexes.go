package server

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ReindexConcurrently reconstrói um índice sem bloquear escritas na tabela —
// mais lento que REINDEX normal, mas seguro rodar em produção. Não pode rodar
// dentro de uma transação, o que já é o caso aqui (uma query por Exec).
func (s *Service) ReindexConcurrently(ctx context.Context, id, database, schema, name string) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
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

	sql := fmt.Sprintf("REINDEX INDEX CONCURRENTLY %s", pgx.Identifier{schema, name}.Sanitize())
	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return nil
}

func (s *Service) DropIndex(ctx context.Context, id, database, schema, name string) error {
	if !identRegex.MatchString(schema) || !identRegex.MatchString(name) {
		return fmt.Errorf("%w: schema/nome inválido", ErrValidation)
	}
	sql := fmt.Sprintf("DROP INDEX %s", pgx.Identifier{schema, name}.Sanitize())
	return s.execDDL(ctx, id, database, sql)
}

// ---------- Sugestão de índices ----------

type IndexSuggestion struct {
	Schema     string `json:"schema"`
	Table      string `json:"table"`
	SeqScan    int64  `json:"seq_scan"`
	SeqTupRead int64  `json:"seq_tup_read"`
	IdxScan    int64  `json:"idx_scan"`
	LiveRows   int64  `json:"live_rows"`
	Detail     string `json:"detail"`
}

// SuggestMissingIndexes é uma heurística barata: tabelas com muito mais leitura
// via seq scan do que via índice, e que já têm volume suficiente pra isso
// importar. Não sabemos QUAIS colunas indexar sem parsear o WHERE das queries
// reais (isso ficaria frágil e fora de escopo) — a sugestão é "olha essa
// tabela", não uma coluna pronta.
func (s *Service) SuggestMissingIndexes(ctx context.Context, id, database string) ([]IndexSuggestion, error) {
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
		SELECT schemaname, relname, seq_scan, seq_tup_read, COALESCE(idx_scan, 0), n_live_tup
		FROM pg_stat_user_tables
		WHERE seq_scan > 0
			AND n_live_tup > 1000
			AND seq_scan > COALESCE(idx_scan, 0)
		ORDER BY seq_tup_read DESC
		LIMIT 20
	`)
	if err != nil {
		return nil, fmt.Errorf("sugerindo índices: %w", err)
	}
	defer rows.Close()

	out := make([]IndexSuggestion, 0)
	for rows.Next() {
		var sug IndexSuggestion
		if err := rows.Scan(&sug.Schema, &sug.Table, &sug.SeqScan, &sug.SeqTupRead, &sug.IdxScan, &sug.LiveRows); err != nil {
			return nil, fmt.Errorf("lendo sugestão: %w", err)
		}
		sug.Detail = fmt.Sprintf(
			"%d seq scans lendo %d linhas no total, contra %d scans via índice — considere indexar as colunas mais usadas em WHERE/JOIN dessa tabela",
			sug.SeqScan, sug.SeqTupRead, sug.IdxScan,
		)
		out = append(out, sug)
	}
	return out, rows.Err()
}

// ---------- Índices não usados ----------

type UnusedIndex struct {
	Schema    string `json:"schema"`
	Table     string `json:"table"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
}

func (s *Service) ListUnusedIndexes(ctx context.Context, id, database string) ([]UnusedIndex, error) {
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
		SELECT s.schemaname, s.relname, s.indexrelname, pg_relation_size(s.indexrelid)
		FROM pg_stat_user_indexes s
		JOIN pg_index ix ON ix.indexrelid = s.indexrelid
		WHERE s.idx_scan = 0 AND NOT ix.indisprimary AND NOT ix.indisunique
		ORDER BY pg_relation_size(s.indexrelid) DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listando índices não usados: %w", err)
	}
	defer rows.Close()

	out := make([]UnusedIndex, 0)
	for rows.Next() {
		var u UnusedIndex
		if err := rows.Scan(&u.Schema, &u.Table, &u.Name, &u.SizeBytes); err != nil {
			return nil, fmt.Errorf("lendo índice não usado: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
