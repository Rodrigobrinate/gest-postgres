package server

import (
	"context"
	"sync"
	"time"

	"github.com/gest-postgres/backend/internal/docker"
)

type PlatformMetricPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryUsedMB   float64   `json:"memory_used_mb"`
	DiskUsedBytes  int64     `json:"disk_used_bytes"`
	NetworkRxBytes int64     `json:"network_rx_bytes"` // cumulativo, igual ao snapshot ao vivo
	NetworkTxBytes int64     `json:"network_tx_bytes"`
	// ReadOpsPerSec/WriteOpsPerSec vêm de /proc/diskstats do host (não soma
	// de container) — operações completadas por segundo, não bytes. 0 na
	// primeira amostra depois do backend subir (docker.HostIOPS precisa de
	// duas leituras pra calcular taxa).
	ReadOpsPerSec  float64 `json:"read_ops_per_sec"`
	WriteOpsPerSec float64 `json:"write_ops_per_sec"`
}

// platformHistory guarda uma janela curta (~1h a 15s/amostra, igual ao
// histórico por servidor) só pra alimentar os sparklines do dashboard — em
// memória, reseta se o backend reiniciar, mesmo trade-off já aceito em
// history.go.
type platformHistory struct {
	mu     sync.Mutex
	points []PlatformMetricPoint
	maxLen int
}

func newPlatformHistory(maxLen int) *platformHistory {
	return &platformHistory{maxLen: maxLen}
}

func (h *platformHistory) append(p PlatformMetricPoint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.points = append(h.points, p)
	if len(h.points) > h.maxLen {
		h.points = h.points[len(h.points)-h.maxLen:]
	}
}

func (h *platformHistory) get() []PlatformMetricPoint {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]PlatformMetricPoint, len(h.points))
	copy(out, h.points)
	return out
}

// GetPlatformStatsHistory é o equivalente pro agregado da plataforma, mesmo
// raciocínio de GetMetricsHistory.
func (s *Service) GetPlatformStatsHistory(ctx context.Context, rangeDur time.Duration) ([]PlatformMetricPoint, error) {
	if rangeDur <= metricHistoryFromDBWindow {
		return s.platformHistory.get(), nil
	}
	return s.getPlatformMetricHistoryDB(ctx, time.Now().Add(-rangeDur))
}

// RunPlatformHistoryCollector roda em background (chamado uma vez no main),
// amostrando os agregados da plataforma a cada `interval`.
func (s *Service) RunPlatformHistoryCollector(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats, err := s.GetPlatformStats(ctx)
			if err != nil {
				continue
			}
			point := PlatformMetricPoint{
				Timestamp:      time.Now(),
				CPUPercent:     stats.TotalCPUPercent,
				MemoryUsedMB:   stats.TotalMemoryUsedMB,
				DiskUsedBytes:  stats.DiskUsedBytes,
				NetworkRxBytes: stats.NetworkRxBytesTotal,
				NetworkTxBytes: stats.NetworkTxBytesTotal,
			}
			// Mesmo tick que já mede tudo o resto — HostIOPS precisa de
			// leitura periódica regular pra calcular taxa por delta (ver
			// docker/hostiops.go), erro na primeira chamada é esperado.
			if readOps, writeOps, err := docker.HostIOPS(); err == nil {
				point.ReadOpsPerSec = readOps
				point.WriteOpsPerSec = writeOps
			}
			s.platformHistory.append(point)
			// Best-effort, mesmo raciocínio de recordServerMetricRaw —
			// sobrevive a reinício do backend.
			s.recordPlatformMetricRaw(ctx, point, stats.TotalMemoryLimitMB)
		}
	}
}
