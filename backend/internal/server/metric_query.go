package server

import (
	"context"
	"encoding/json"
	"time"
)

// metricHistoryFromDBWindow é o teto a partir do qual GetMetricsHistory/
// GetPlatformStatsHistory saem do buffer em memória (rápido, sem round-trip
// no banco, cobre a última ~1h) e passam a consultar metric_history —
// tanto raw (últimas 24h) quanto hourly (mais velho) numa query só: os dois
// nunca se sobrepõem no tempo (o job de rollup garante isso), então um
// filtro simples por bucket_start >= $since já devolve a mistura certa sem
// precisar decidir resolução na mão.
const metricHistoryFromDBWindow = time.Hour

func (s *Service) getServerMetricHistoryDB(ctx context.Context, serverID string, since time.Time) ([]MetricPoint, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT bucket_start, cpu_percent_avg, memory_used_mb_avg, connection_count_avg,
		       disk_used_mb_avg, database_sizes_mb, connections_by_database
		FROM metric_history
		WHERE scope = 'server' AND server_id = $1 AND bucket_start >= $2
		ORDER BY bucket_start
	`, serverID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]MetricPoint, 0)
	for rows.Next() {
		var p MetricPoint
		var connAvg float64
		var dbSizesJSON, connsJSON []byte
		if err := rows.Scan(&p.Timestamp, &p.CPUPercent, &p.MemoryUsedMB, &connAvg, &p.DiskUsedMB, &dbSizesJSON, &connsJSON); err != nil {
			return nil, err
		}
		p.ConnectionCount = int(connAvg)
		if dbSizesJSON != nil {
			_ = json.Unmarshal(dbSizesJSON, &p.DatabaseSizesMB)
		}
		if connsJSON != nil {
			_ = json.Unmarshal(connsJSON, &p.ConnectionsByDatabase)
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

func (s *Service) getPlatformMetricHistoryDB(ctx context.Context, since time.Time) ([]PlatformMetricPoint, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT bucket_start, cpu_percent_avg, memory_used_mb_avg, disk_used_mb_avg,
		       network_rx_bytes, network_tx_bytes, read_ops_per_sec_avg, write_ops_per_sec_avg
		FROM metric_history
		WHERE scope = 'platform' AND bucket_start >= $1
		ORDER BY bucket_start
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]PlatformMetricPoint, 0)
	for rows.Next() {
		var p PlatformMetricPoint
		var diskUsedMB float64
		if err := rows.Scan(&p.Timestamp, &p.CPUPercent, &p.MemoryUsedMB, &diskUsedMB, &p.NetworkRxBytes, &p.NetworkTxBytes, &p.ReadOpsPerSec, &p.WriteOpsPerSec); err != nil {
			return nil, err
		}
		p.DiskUsedBytes = int64(diskUsedMB * 1024 * 1024)
		points = append(points, p)
	}
	return points, rows.Err()
}

// ParseHistoryRange traduz o parâmetro `?range=` da API pra uma janela —
// "" (default) usa só o buffer em memória (comportamento de sempre, sem ir
// no banco); qualquer valor reconhecido maior que 1h vai buscar em
// metric_history. Valor não reconhecido cai pro default, silenciosamente
// (não é uma validação que vale a pena travar a requisição por causa dela).
func ParseHistoryRange(raw string) time.Duration {
	switch raw {
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return metricHistoryFromDBWindow
	}
}
