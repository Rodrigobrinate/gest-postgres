package infra

import (
	"context"
	"sync"
	"time"
)

type ContainerMetricPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryUsedMB   float64   `json:"memory_used_mb"`
	NetworkRxBytes int64     `json:"network_rx_bytes"`
	NetworkTxBytes int64     `json:"network_tx_bytes"`
}

const (
	containerHistoryMaxLen   = 240 // ~1h a 15s/amostra, mesma janela do resto da plataforma
	containerHistoryInterval = 15 * time.Second
	// "todo container do host" é um conjunto sem limite, diferente dos
	// servidores gerenciados (sempre coletados) — um coletor por container
	// pra sempre desperdiçaria goroutine/CPU pra containers que ninguém
	// olha. Cada coletor se auto-encerra depois desse tempo sem leitura.
	containerHistoryIdleTTL = 10 * time.Minute
)

type containerHistory struct {
	mu       sync.Mutex
	points   []ContainerMetricPoint
	lastRead time.Time
}

func (h *containerHistory) append(p ContainerMetricPoint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.points = append(h.points, p)
	if len(h.points) > containerHistoryMaxLen {
		h.points = h.points[len(h.points)-containerHistoryMaxLen:]
	}
}

func (h *containerHistory) get() []ContainerMetricPoint {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastRead = time.Now()
	out := make([]ContainerMetricPoint, len(h.points))
	copy(out, h.points)
	return out
}

func (h *containerHistory) idleFor() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	return time.Since(h.lastRead)
}

type containerHistories struct {
	mu    sync.Mutex
	items map[string]*containerHistory
}

func newContainerHistories() *containerHistories {
	return &containerHistories{items: make(map[string]*containerHistory)}
}

// ContainerStatsHistory devolve o histórico curto de um container e, na
// primeira leitura, dispara o coletor em background pra esse container
// (idempotente — leituras seguintes reusam o mesmo coletor).
func (s *Service) ContainerStatsHistory(ctx context.Context, containerID string) []ContainerMetricPoint {
	s.containerHistories.mu.Lock()
	h, exists := s.containerHistories.items[containerID]
	if !exists {
		h = &containerHistory{lastRead: time.Now()}
		s.containerHistories.items[containerID] = h
	}
	s.containerHistories.mu.Unlock()

	if !exists {
		go s.collectContainerHistory(containerID, h)
	}
	return h.get()
}

func (s *Service) collectContainerHistory(containerID string, h *containerHistory) {
	ticker := time.NewTicker(containerHistoryInterval)
	defer ticker.Stop()
	ctx := context.Background()

	for range ticker.C {
		if h.idleFor() > containerHistoryIdleTTL {
			s.containerHistories.mu.Lock()
			delete(s.containerHistories.items, containerID)
			s.containerHistories.mu.Unlock()
			return
		}
		snap, err := s.docker.ContainerStats(ctx, containerID)
		if err != nil {
			continue
		}
		h.append(ContainerMetricPoint{
			Timestamp:      time.Now(),
			CPUPercent:     snap.CPUPercent,
			MemoryUsedMB:   snap.MemoryUsedMB,
			NetworkRxBytes: snap.NetworkRxBytes,
			NetworkTxBytes: snap.NetworkTxBytes,
		})
	}
}
