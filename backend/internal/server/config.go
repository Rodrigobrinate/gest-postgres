package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// gucRestartRequired é fixo — quais GUCs do subset do MVP exigem restart do
// postmaster pra valer (o resto só precisa de reload, que ApplyConfig já faz
// sozinho). Não muda entre versões menores do Postgres.
var gucRestartRequired = map[string]bool{
	"max_connections": true,
	"shared_buffers":  true,
}

type LiveConfig struct {
	PostgresConfig
	RestartPending bool `json:"restart_pending"`
}

// GetLiveConfig lê os valores REAIS aplicados no servidor agora (via
// pg_settings), não o que ficou gravado no metadata DB na criação — o usuário
// pode ter mudado por fora, ou uma mudança pode estar pendente de restart.
func (s *Service) GetLiveConfig(ctx context.Context, id, database string) (*LiveConfig, error) {
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
		SELECT name, setting, unit, pending_restart
		FROM pg_settings
		WHERE name = ANY($1)
	`, []string{
		"max_connections", "shared_buffers", "work_mem",
		"maintenance_work_mem", "effective_cache_size", "log_min_duration_statement",
	})
	if err != nil {
		return nil, fmt.Errorf("lendo pg_settings: %w", err)
	}
	defer rows.Close()

	cfg := &LiveConfig{}
	for rows.Next() {
		var name, setting string
		var unitPtr *string // pg_settings.unit é NULL pra GUCs sem unidade (ex: max_connections)
		var pending bool
		if err := rows.Scan(&name, &setting, &unitPtr, &pending); err != nil {
			return nil, fmt.Errorf("lendo parâmetro: %w", err)
		}
		if pending {
			cfg.RestartPending = true
		}
		unit := ""
		if unitPtr != nil {
			unit = *unitPtr
		}

		switch name {
		case "max_connections":
			cfg.MaxConnections = atoiSafe(setting)
		case "shared_buffers":
			cfg.SharedBuffersMB = toMB(setting, unit)
		case "work_mem":
			cfg.WorkMemMB = toMB(setting, unit)
		case "maintenance_work_mem":
			cfg.MaintenanceWorkMemMB = toMB(setting, unit)
		case "effective_cache_size":
			cfg.EffectiveCacheSizeMB = toMB(setting, unit)
		case "log_min_duration_statement":
			cfg.LogMinDurationStatementMs = toMs(setting, unit)
		}
	}
	return cfg, rows.Err()
}

// ApplyConfig aplica o subset de postgresql.conf via ALTER SYSTEM + reload.
// Retorna true se algum parâmetro alterado só pega valer depois de restart
// (max_connections, shared_buffers) — nesse caso o reload já rodou, mas o
// valor antigo continua valendo até reiniciar o container.
func (s *Service) ApplyConfig(ctx context.Context, id, database string, cfg PostgresConfig) (bool, error) {
	record, err := s.getRunningServer(ctx, id)
	if err != nil {
		return false, err
	}
	return s.applySettings(ctx, record, database, cfg)
}

// applySettings é a versão sem checagem de status — usada pelo provisionamento
// inicial (server.provision), quando o container já aceita conexão mas o
// status no metadata DB ainda é "creating", não "running".
func (s *Service) applySettings(ctx context.Context, record *Server, database string, cfg PostgresConfig) (bool, error) {
	conn, err := s.connectTo(ctx, record, database)
	if err != nil {
		return false, err
	}
	defer conn.Close(ctx)

	settings := map[string]string{
		"max_connections":             strconv.Itoa(cfg.MaxConnections),
		"shared_buffers":              fmt.Sprintf("%dMB", cfg.SharedBuffersMB),
		"work_mem":                    fmt.Sprintf("%dMB", cfg.WorkMemMB),
		"maintenance_work_mem":        fmt.Sprintf("%dMB", cfg.MaintenanceWorkMemMB),
		"effective_cache_size":        fmt.Sprintf("%dMB", cfg.EffectiveCacheSizeMB),
		"log_min_duration_statement":  strconv.Itoa(cfg.LogMinDurationStatementMs),
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

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func toMB(setting, unit string) int {
	n, err := strconv.ParseFloat(setting, 64)
	if err != nil {
		return 0
	}
	switch unit {
	case "8kB":
		return int(n * 8 / 1024)
	case "kB":
		return int(n / 1024)
	case "MB":
		return int(n)
	case "GB":
		return int(n * 1024)
	default:
		return int(n)
	}
}

func toMs(setting, unit string) int {
	n, err := strconv.ParseFloat(setting, 64)
	if err != nil {
		return 0
	}
	switch unit {
	case "s":
		return int(n * 1000)
	case "min":
		return int(n * 60000)
	case "ms", "":
		return int(n)
	default:
		return int(n)
	}
}
