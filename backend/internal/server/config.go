package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// gucRestartRequired é fixo — quais dos 6 GUCs calculados pelo preset de
// criação exigem restart do postmaster pra valer. Não muda entre versões
// menores do Postgres.
var gucRestartRequired = map[string]bool{
	"max_connections": true,
	"shared_buffers":  true,
}

// applySettings aplica o subset de 6 parâmetros calculado pelo preset de
// recursos (ver presets.go) via ALTER SYSTEM + reload — usado só no
// provisionamento inicial (server.provision), quando o container já aceita
// conexão mas o status no metadata DB ainda é "creating", não "running". Pra
// edição manual pós-criação de qualquer um dos ~80 parâmetros suportados, ver
// expanded_config.go.
func (s *Service) applySettings(ctx context.Context, record *Server, database string, cfg PostgresConfig) (bool, error) {
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return false, err
	}
	defer conn.Close(ctx)

	settings := map[string]string{
		"max_connections":            strconv.Itoa(cfg.MaxConnections),
		"shared_buffers":             fmt.Sprintf("%dMB", cfg.SharedBuffersMB),
		"work_mem":                   fmt.Sprintf("%dMB", cfg.WorkMemMB),
		"maintenance_work_mem":       fmt.Sprintf("%dMB", cfg.MaintenanceWorkMemMB),
		"effective_cache_size":       fmt.Sprintf("%dMB", cfg.EffectiveCacheSizeMB),
		"log_min_duration_statement": strconv.Itoa(cfg.LogMinDurationStatementMs),
	}

	restartRequired := false
	for name, value := range settings {
		sql := fmt.Sprintf("ALTER SYSTEM SET %s = %s", name, sqlQuoteLiteral(value))
		if _, err := conn.Exec(ctx, sql); err != nil {
			return false, fmt.Errorf("%w: aplicando %s: %v", ErrValidation, name, err)
		}
		if gucRestartRequired[name] {
			restartRequired = true
		}
	}

	if _, err := conn.Exec(ctx, "SELECT pg_reload_conf()"); err != nil {
		return false, fmt.Errorf("recarregando config: %w", err)
	}

	return restartRequired, nil
}

func sqlQuoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
