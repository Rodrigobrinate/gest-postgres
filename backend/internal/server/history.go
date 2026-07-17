package server

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type MetricPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	CPUPercent      float64   `json:"cpu_percent"`
	MemoryUsedMB    float64   `json:"memory_used_mb"`
	ConnectionCount int       `json:"connection_count"`
}

// HistoryCollector guarda uma janela recente de métricas por servidor, só em
// memória — reseta se o backend reiniciar. Suficiente pro MVP (gráfico "última
// hora"); virar série temporal persistida é backlog (REQUISITOS.md §7).
type HistoryCollector struct {
	mu     sync.Mutex
	points map[string][]MetricPoint
	maxLen int
}

func NewHistoryCollector(maxLen int) *HistoryCollector {
	return &HistoryCollector{points: make(map[string][]MetricPoint), maxLen: maxLen}
}

func (h *HistoryCollector) append(serverID string, p MetricPoint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	pts := append(h.points[serverID], p)
	if len(pts) > h.maxLen {
		pts = pts[len(pts)-h.maxLen:]
	}
	h.points[serverID] = pts
}

func (h *HistoryCollector) get(serverID string) []MetricPoint {
	h.mu.Lock()
	defer h.mu.Unlock()
	pts := h.points[serverID]
	out := make([]MetricPoint, len(pts))
	copy(out, pts)
	return out
}

func (h *HistoryCollector) forget(serverID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.points, serverID)
}

func (s *Service) GetMetricsHistory(id string) []MetricPoint {
	return s.history.get(id)
}

// RunMetricsCollector roda em background (chamado uma vez no main) amostrando
// CPU/mem/conexões de todo servidor "running" a cada `interval`. Best-effort:
// erro em um servidor não afeta os outros nem para o loop.
func (s *Service) RunMetricsCollector(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collectMetricsOnce(ctx)
		}
	}
}

func (s *Service) collectMetricsOnce(ctx context.Context) {
	records, err := s.repo.List(ctx)
	if err != nil {
		return
	}

	now := time.Now()
	for _, record := range records {
		if record.Status != StatusRunning || record.ContainerID == "" {
			continue
		}

		stats, err := s.docker.ContainerStats(ctx, record.ContainerID)
		if err != nil {
			slog.Warn("coleta de métricas: falha lendo stats do container", "server_id", record.ID, "error", err)
			continue
		}

		connCount, err := s.countConnections(ctx, record)
		if err != nil {
			slog.Warn("coleta de métricas: falha contando conexões", "server_id", record.ID, "error", err)
			connCount = -1
		}

		s.history.append(record.ID, MetricPoint{
			Timestamp:       now,
			CPUPercent:      stats.CPUPercent,
			MemoryUsedMB:    stats.MemoryUsedMB,
			ConnectionCount: connCount,
		})
	}
}

func (s *Service) countConnections(ctx context.Context, record *Server) (int, error) {
	conn, err := s.connectTo(ctx, record, record.DatabaseName)
	if err != nil {
		return 0, err
	}
	defer conn.Close(ctx)

	var count int
	err = conn.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE backend_type = 'client backend'`).Scan(&count)
	return count, err
}
