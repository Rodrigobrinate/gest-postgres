package server

import (
	"context"
	"log/slog"
	"time"
)

const (
	// rawRetentionWindow: dado raw (~15s/amostra) só existe pras últimas
	// 24h — depois disso vira "hourly" (avg/min/max por hora) e o raw é
	// apagado. 24h é "o que é relevante" segundo o próprio pedido do
	// usuário; além disso, o valor que importa é saber que dia teve um
	// pico, não o segundo exato.
	rawRetentionWindow = 24 * time.Hour
	// hourlyRetentionWindow: retenção bem mais longa pro dado já resumido
	// — barato o bastante (1 linha/hora, não 1 linha/15s) pra manter por
	// meses sem preocupação de tamanho de banco. Ainda assim tem um teto
	// (não "pra sempre") — mesmo espírito de retention_count do backup.
	hourlyRetentionWindow = 180 * 24 * time.Hour
	rollupInterval        = 30 * time.Minute
)

// RunMetricRollup roda em background (chamado uma vez no main): agrega raw
// mais velho que 24h em linhas "hourly" (avg/min/max), apaga o raw já
// agregado, e aplica a retenção do hourly. Idempotente — rodar de novo
// sobre o mesmo intervalo só sobrescreve o mesmo bucket (ON CONFLICT DO
// UPDATE), então uma falha no meio do caminho não deixa lixo nem
// duplicata, só tenta de novo no próximo tick.
func (s *Service) RunMetricRollup(ctx context.Context) {
	s.rollupMetricsOnce(ctx)

	ticker := time.NewTicker(rollupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.rollupMetricsOnce(ctx)
		}
	}
}

func (s *Service) rollupMetricsOnce(ctx context.Context) {
	rawCutoff := time.Now().Add(-rawRetentionWindow)
	hourlyCutoff := time.Now().Add(-hourlyRetentionWindow)

	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		slog.Error("rollup de métricas: falha abrindo transação", "error", err)
		return
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO metric_history (
			scope, server_id, resolution, bucket_start,
			cpu_percent_avg, cpu_percent_min, cpu_percent_max,
			memory_used_mb_avg, memory_used_mb_min, memory_used_mb_max,
			memory_limit_mb,
			disk_used_mb_avg, disk_used_mb_max,
			network_rx_bytes, network_tx_bytes,
			read_ops_per_sec_avg, read_ops_per_sec_max,
			write_ops_per_sec_avg, write_ops_per_sec_max
		)
		SELECT
			'platform', NULL, 'hourly', date_trunc('hour', bucket_start),
			avg(cpu_percent_avg), min(cpu_percent_min), max(cpu_percent_max),
			avg(memory_used_mb_avg), min(memory_used_mb_min), max(memory_used_mb_max),
			max(memory_limit_mb),
			avg(disk_used_mb_avg), max(disk_used_mb_max),
			max(network_rx_bytes), max(network_tx_bytes),
			avg(read_ops_per_sec_avg), max(read_ops_per_sec_max),
			avg(write_ops_per_sec_avg), max(write_ops_per_sec_max)
		FROM metric_history
		WHERE scope = 'platform' AND resolution = 'raw' AND bucket_start < $1
		GROUP BY date_trunc('hour', bucket_start)
		ON CONFLICT (resolution, bucket_start) WHERE scope = 'platform' DO UPDATE SET
			cpu_percent_avg = EXCLUDED.cpu_percent_avg,
			cpu_percent_min = EXCLUDED.cpu_percent_min,
			cpu_percent_max = EXCLUDED.cpu_percent_max,
			memory_used_mb_avg = EXCLUDED.memory_used_mb_avg,
			memory_used_mb_min = EXCLUDED.memory_used_mb_min,
			memory_used_mb_max = EXCLUDED.memory_used_mb_max,
			memory_limit_mb = EXCLUDED.memory_limit_mb,
			disk_used_mb_avg = EXCLUDED.disk_used_mb_avg,
			disk_used_mb_max = EXCLUDED.disk_used_mb_max,
			network_rx_bytes = EXCLUDED.network_rx_bytes,
			network_tx_bytes = EXCLUDED.network_tx_bytes,
			read_ops_per_sec_avg = EXCLUDED.read_ops_per_sec_avg,
			read_ops_per_sec_max = EXCLUDED.read_ops_per_sec_max,
			write_ops_per_sec_avg = EXCLUDED.write_ops_per_sec_avg,
			write_ops_per_sec_max = EXCLUDED.write_ops_per_sec_max
	`, rawCutoff); err != nil {
		slog.Error("rollup de métricas: falha agregando platform", "error", err)
		return
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO metric_history (
			scope, server_id, resolution, bucket_start,
			cpu_percent_avg, cpu_percent_min, cpu_percent_max,
			memory_used_mb_avg, memory_used_mb_min, memory_used_mb_max,
			connection_count_avg, connection_count_max,
			disk_used_mb_avg, disk_used_mb_max
		)
		SELECT
			'server', server_id, 'hourly', date_trunc('hour', bucket_start),
			avg(cpu_percent_avg), min(cpu_percent_min), max(cpu_percent_max),
			avg(memory_used_mb_avg), min(memory_used_mb_min), max(memory_used_mb_max),
			avg(connection_count_avg), max(connection_count_max),
			avg(disk_used_mb_avg), max(disk_used_mb_max)
		FROM metric_history
		WHERE scope = 'server' AND resolution = 'raw' AND bucket_start < $1
		GROUP BY server_id, date_trunc('hour', bucket_start)
		ON CONFLICT (server_id, resolution, bucket_start) WHERE scope = 'server' DO UPDATE SET
			cpu_percent_avg = EXCLUDED.cpu_percent_avg,
			cpu_percent_min = EXCLUDED.cpu_percent_min,
			cpu_percent_max = EXCLUDED.cpu_percent_max,
			memory_used_mb_avg = EXCLUDED.memory_used_mb_avg,
			memory_used_mb_min = EXCLUDED.memory_used_mb_min,
			memory_used_mb_max = EXCLUDED.memory_used_mb_max,
			connection_count_avg = EXCLUDED.connection_count_avg,
			connection_count_max = EXCLUDED.connection_count_max,
			disk_used_mb_avg = EXCLUDED.disk_used_mb_avg,
			disk_used_mb_max = EXCLUDED.disk_used_mb_max
	`, rawCutoff); err != nil {
		slog.Error("rollup de métricas: falha agregando server", "error", err)
		return
	}

	if _, err := tx.Exec(ctx, `DELETE FROM metric_history WHERE resolution = 'raw' AND bucket_start < $1`, rawCutoff); err != nil {
		slog.Error("rollup de métricas: falha apagando raw agregado", "error", err)
		return
	}

	if _, err := tx.Exec(ctx, `DELETE FROM metric_history WHERE resolution = 'hourly' AND bucket_start < $1`, hourlyCutoff); err != nil {
		slog.Error("rollup de métricas: falha aplicando retenção do hourly", "error", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("rollup de métricas: falha no commit", "error", err)
	}
}
