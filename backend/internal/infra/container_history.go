package infra

import (
	"context"
	"log/slog"
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
)

type containerHistory struct {
	mu     sync.Mutex
	points []ContainerMetricPoint
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
	out := make([]ContainerMetricPoint, len(h.points))
	copy(out, h.points)
	return out
}

type containerHistories struct {
	mu    sync.Mutex
	items map[string]*containerHistory
}

func newContainerHistories() *containerHistories {
	return &containerHistories{items: make(map[string]*containerHistory)}
}

// ContainerStatsHistory devolve o histórico de um container — coletado em
// background pra TODO container do host (ver RunContainerMetricsCollector),
// igual ao histórico de servidor Postgres gerenciado, não mais sob demanda
// só quando alguém abre a aba Estatísticas. Container que ainda não
// completou a primeira amostra (acabou de subir, ou o coletor de fundo
// ainda não rodou) devolve vazio, não erro.
func (s *Service) ContainerStatsHistory(ctx context.Context, containerID string) []ContainerMetricPoint {
	s.containerHistories.mu.Lock()
	h, exists := s.containerHistories.items[containerID]
	s.containerHistories.mu.Unlock()
	if !exists {
		return nil
	}
	return h.get()
}

// RunContainerMetricsCollector roda em background (chamado uma vez no main,
// mesmo padrão do RunMetricsCollector de servidor Postgres) — amostra
// CPU/mem/rede de TODO container rodando no host a cada `interval`, sempre
// ligado, independente de alguém ter aberto a aba Estatísticas daquele
// container. Histórico de container que parou/sumiu do host é descartado no
// mesmo ciclo (evita vazar memória pra sempre num host com bastante
// rotatividade de container).
func (s *Service) RunContainerMetricsCollector(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collectAllContainerHistory(ctx)
		}
	}
}

func (s *Service) collectAllContainerHistory(ctx context.Context) {
	containers, err := s.docker.ListAllContainers(ctx)
	if err != nil {
		slog.Warn("coleta de histórico de container: falha listando containers", "error", err)
		return
	}

	running := make(map[string]bool, len(containers))
	var wg sync.WaitGroup
	for _, c := range containers {
		if c.State != "running" {
			continue
		}
		running[c.ID] = true

		s.containerHistories.mu.Lock()
		h, exists := s.containerHistories.items[c.ID]
		if !exists {
			h = &containerHistory{}
			s.containerHistories.items[c.ID] = h
		}
		s.containerHistories.mu.Unlock()

		wg.Add(1)
		go func(containerID string, h *containerHistory) {
			defer wg.Done()
			collectOnce(ctx, s, containerID, h)
		}(c.ID, h)
	}
	wg.Wait()

	s.containerHistories.mu.Lock()
	for id := range s.containerHistories.items {
		if !running[id] {
			delete(s.containerHistories.items, id)
		}
	}
	s.containerHistories.mu.Unlock()
}

// collectOnce lê um snapshot de stats e adiciona ao histórico.
func collectOnce(parent context.Context, s *Service, containerID string, h *containerHistory) {
	// Timeout por chamada — sem isso, uma chamada de stats que trava (proxy
	// lento, container num estado estranho) empaca esse goroutine pra
	// sempre, sem erro nenhum: nunca mais nenhum ponto entra no histórico,
	// silenciosamente, porque o loop nunca volta pro ticker.
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	snap, err := s.docker.ContainerStats(ctx, containerID)
	if err != nil {
		slog.Error("falha coletando stats de container pro histórico", "error", err, "container_id", containerID)
		return
	}
	h.append(ContainerMetricPoint{
		Timestamp:      time.Now(),
		CPUPercent:     snap.CPUPercent,
		MemoryUsedMB:   snap.MemoryUsedMB,
		NetworkRxBytes: snap.NetworkRxBytes,
		NetworkTxBytes: snap.NetworkTxBytes,
	})
}
