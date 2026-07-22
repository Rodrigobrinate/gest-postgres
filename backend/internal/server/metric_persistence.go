package server

import (
	"context"
	"encoding/json"
	"fmt"
)

// recordServerMetricRaw grava a amostra atual de um servidor na tabela
// persistida — chamado no MESMO tick que já alimenta o HistoryCollector em
// memória (ver history.go), então o dado passa a sobreviver a reinício do
// backend sem duplicar a lógica de coleta. avg=min=max na linha raw (uma
// amostra só) — a distinção só importa nas linhas "hourly" que o job de
// rollup gera depois.
func (s *Service) recordServerMetricRaw(ctx context.Context, serverID string, p MetricPoint) {
	dbSizesJSON, err := marshalOrNil(p.DatabaseSizesMB)
	if err != nil {
		return
	}
	connsJSON, err := marshalOrNil(p.ConnectionsByDatabase)
	if err != nil {
		return
	}

	_, _ = s.repo.pool.Exec(ctx, `
		INSERT INTO metric_history (
			scope, server_id, resolution, bucket_start,
			cpu_percent_avg, cpu_percent_min, cpu_percent_max,
			memory_used_mb_avg, memory_used_mb_min, memory_used_mb_max,
			connection_count_avg, connection_count_max,
			disk_used_mb_avg, disk_used_mb_max,
			database_sizes_mb, connections_by_database
		) VALUES (
			'server', $1, 'raw', $2,
			$3, $3, $3,
			$4, $4, $4,
			$5, $6,
			$7, $7,
			$8, $9
		)
		ON CONFLICT (server_id, resolution, bucket_start) WHERE scope = 'server' DO NOTHING
	`, serverID, p.Timestamp, p.CPUPercent, p.MemoryUsedMB, float64(p.ConnectionCount), p.ConnectionCount, p.DiskUsedMB, dbSizesJSON, connsJSON)
}

// recordPlatformMetricRaw é o equivalente pro agregado da plataforma (os 4
// cards do dashboard) — mesmo raciocínio de recordServerMetricRaw.
func (s *Service) recordPlatformMetricRaw(ctx context.Context, p PlatformMetricPoint, memoryLimitMB float64) {
	_, _ = s.repo.pool.Exec(ctx, `
		INSERT INTO metric_history (
			scope, server_id, resolution, bucket_start,
			cpu_percent_avg, cpu_percent_min, cpu_percent_max,
			memory_used_mb_avg, memory_used_mb_min, memory_used_mb_max,
			memory_limit_mb,
			disk_used_mb_avg, disk_used_mb_max,
			network_rx_bytes, network_tx_bytes,
			read_ops_per_sec_avg, read_ops_per_sec_max,
			write_ops_per_sec_avg, write_ops_per_sec_max
		) VALUES (
			'platform', NULL, 'raw', $1,
			$2, $2, $2,
			$3, $3, $3,
			$4,
			$5, $5,
			$6, $7,
			$8, $8,
			$9, $9
		)
		ON CONFLICT (resolution, bucket_start) WHERE scope = 'platform' DO NOTHING
	`, p.Timestamp, p.CPUPercent, p.MemoryUsedMB, memoryLimitMB, float64(p.DiskUsedBytes)/(1024*1024),
		p.NetworkRxBytes, p.NetworkTxBytes, p.ReadOpsPerSec, p.WriteOpsPerSec)
}

func marshalOrNil[T any](m map[string]T) ([]byte, error) {
	if len(m) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("serializando breakdown por banco: %w", err)
	}
	return b, nil
}
