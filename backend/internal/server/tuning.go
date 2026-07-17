package server

import (
	"context"
	"fmt"
	"strings"
)

type TuningSuggestion struct {
	Param          string `json:"param"`
	CurrentValue   string `json:"current_value"`
	SuggestedValue string `json:"suggested_value"`
	Reason         string `json:"reason"`
	Differs        bool   `json:"differs"`
}

// SuggestTuning reaproveita as mesmas regras de bolso do ConfigForResources
// (presets.go) — shared_buffers/work_mem/etc a partir da RAM do container —
// e acrescenta autovacuum e I/O, comparando com o valor ao vivo do servidor
// pra destacar o que realmente vale a pena mudar. Assume volume em SSD (é o
// padrão em qualquer provedor cloud atual) pra random_page_cost/io_concurrency.
func (s *Service) SuggestTuning(ctx context.Context, id string) ([]TuningSuggestion, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.connectTo(ctx, record, "")
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	cfg := ConfigForResources(record.Resources)

	cores := int(record.Resources.CPUCores)
	autovacuumWorkers := 3
	if cores > 2 {
		autovacuumWorkers = 3 + (cores-2)/4
		if autovacuumWorkers > 6 {
			autovacuumWorkers = 6
		}
	}

	suggestions := []TuningSuggestion{
		{
			Param:          "shared_buffers",
			SuggestedValue: fmt.Sprintf("%dMB", cfg.SharedBuffersMB),
			Reason:         "~25% da RAM do container — ponto de partida padrão pra cache do Postgres",
		},
		{
			Param:          "effective_cache_size",
			SuggestedValue: fmt.Sprintf("%dMB", cfg.EffectiveCacheSizeMB),
			Reason:         "~75% da RAM — sinaliza ao planner quanto cache (Postgres + SO) tá disponível",
		},
		{
			Param:          "work_mem",
			SuggestedValue: fmt.Sprintf("%dMB", cfg.WorkMemMB),
			Reason:         "RAM dividida por max_connections — conservador pra não estourar sob carga concorrente",
		},
		{
			Param:          "maintenance_work_mem",
			SuggestedValue: fmt.Sprintf("%dMB", cfg.MaintenanceWorkMemMB),
			Reason:         "usado por VACUUM/CREATE INDEX — pode ser bem maior que work_mem, não é por conexão",
		},
		{
			Param:          "max_connections",
			SuggestedValue: fmt.Sprintf("%d", cfg.MaxConnections),
			Reason:         "baseado na RAM disponível — cada conexão tem overhead fixo de memória",
		},
		{
			Param:          "effective_io_concurrency",
			SuggestedValue: "200",
			Reason:         "assume disco SSD (padrão em provedores cloud) — permite mais leituras paralelas",
		},
		{
			Param:          "random_page_cost",
			SuggestedValue: "1.1",
			Reason:         "assume disco SSD — custo de acesso aleatório muito menor que o padrão pensado pra HD",
		},
		{
			Param:          "autovacuum_max_workers",
			SuggestedValue: fmt.Sprintf("%d", autovacuumWorkers),
			Reason:         fmt.Sprintf("escalado com %d vCPU(s) do container", cores),
		},
		{
			Param:          "autovacuum_vacuum_cost_limit",
			SuggestedValue: "2000",
			Reason:         "em SSD o autovacuum pode ser bem mais agressivo sem competir tanto por I/O (padrão é 200)",
		},
	}

	names := make([]string, len(suggestions))
	for i, sug := range suggestions {
		names[i] = sug.Param
	}

	rows, err := conn.Query(ctx, `SELECT name, current_setting(name) FROM pg_settings WHERE name = ANY($1)`, names)
	if err != nil {
		return nil, fmt.Errorf("lendo config atual: %w", err)
	}
	defer rows.Close()

	current := make(map[string]string, len(names))
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("lendo parâmetro atual: %w", err)
		}
		current[name] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range suggestions {
		cur := current[suggestions[i].Param]
		suggestions[i].CurrentValue = cur
		suggestions[i].Differs = !strings.EqualFold(
			strings.ReplaceAll(cur, " ", ""),
			strings.ReplaceAll(suggestions[i].SuggestedValue, " ", ""),
		)
	}

	return suggestions, nil
}
