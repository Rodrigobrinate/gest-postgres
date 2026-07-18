package server

import (
	"context"
	"sync"
	"time"
)

type PlatformMetricPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryUsedMB   float64   `json:"memory_used_mb"`
	DiskUsedBytes  int64     `json:"disk_used_bytes"`
	NetworkRxBytes int64     `json:"network_rx_bytes"` // cumulativo, igual ao snapshot ao vivo
	NetworkTxBytes int64     `json:"network_tx_bytes"`
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

func (s *Service) GetPlatformStatsHistory() []PlatformMetricPoint {
	return s.platformHistory.get()
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
			s.platformHistory.append(PlatformMetricPoint{
				Timestamp:      time.Now(),
				CPUPercent:     stats.TotalCPUPercent,
				MemoryUsedMB:   stats.TotalMemoryUsedMB,
				DiskUsedBytes:  stats.DiskUsedBytes,
				NetworkRxBytes: stats.NetworkRxBytesTotal,
				NetworkTxBytes: stats.NetworkTxBytesTotal,
			})
		}
	}
}
