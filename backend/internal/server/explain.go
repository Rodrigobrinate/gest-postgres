package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type ExplainResult struct {
	Plan            json.RawMessage `json:"plan"`
	PlanningTimeMs  *float64        `json:"planning_time_ms,omitempty"`
	ExecutionTimeMs *float64        `json:"execution_time_ms,omitempty"`
}

// ExplainQuery roda EXPLAIN (FORMAT JSON) na query informada. Com analyze=true
// a query É EXECUTADA de verdade (ANALYZE roda o plano pra medir tempo real) —
// mesma fronteira de confiança do editor SQL, o usuário já pode rodar
// qualquer coisa por lá.
func (s *Service) ExplainQuery(ctx context.Context, id, database, sql string, analyze bool) (*ExplainResult, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	trimmed := strings.TrimSpace(sql)
	trimmed = strings.TrimSuffix(trimmed, ";")
	if trimmed == "" {
		return nil, fmt.Errorf("%w: query vazia", ErrValidation)
	}

	opts := "FORMAT JSON"
	if analyze {
		opts = "FORMAT JSON, ANALYZE, BUFFERS"
	}
	explainSQL := fmt.Sprintf("EXPLAIN (%s) %s", opts, trimmed)

	var raw string
	if err := conn.QueryRow(ctx, explainSQL).Scan(&raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	var parsed []struct {
		Plan            json.RawMessage `json:"Plan"`
		PlanningTimeMs  *float64        `json:"Planning Time"`
		ExecutionTimeMs *float64        `json:"Execution Time"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil || len(parsed) == 0 {
		return nil, fmt.Errorf("%w: resposta inesperada do EXPLAIN", ErrValidation)
	}

	return &ExplainResult{
		Plan:            parsed[0].Plan,
		PlanningTimeMs:  parsed[0].PlanningTimeMs,
		ExecutionTimeMs: parsed[0].ExecutionTimeMs,
	}, nil
}
