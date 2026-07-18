package server

import (
	"context"
	"sort"
	"strings"
	"sync"
)

type ContainerStat struct {
	ContainerID    string  `json:"container_id"`
	Name           string  `json:"name"`
	Image          string  `json:"image"`
	IsManaged      bool    `json:"is_managed"`
	ServerID       string  `json:"server_id,omitempty"`
	ServerName     string  `json:"server_name,omitempty"`
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryUsedMB   float64 `json:"memory_used_mb"`
	MemoryLimitMB  float64 `json:"memory_limit_mb"`
	NetworkRxBytes int64   `json:"network_rx_bytes"`
	NetworkTxBytes int64   `json:"network_tx_bytes"`
}

// PlatformStats agrega TODOS os containers Docker do host (não só os
// gerenciados) — é o proxy honesto de "recursos da plataforma" que dá pra
// ter sem acesso ao host além da API Docker (sem host real CPU/mem/disco/rede,
// já que o backend roda dentro de container e só fala com o Docker via
// socket-proxy de propósito).
type PlatformStats struct {
	Containers          []ContainerStat `json:"containers"`
	TotalCPUPercent     float64         `json:"total_cpu_percent"`
	TotalMemoryUsedMB   float64         `json:"total_memory_used_mb"`
	TotalMemoryLimitMB  float64         `json:"total_memory_limit_mb"`
	DiskUsedBytes       int64           `json:"disk_used_bytes"`
	NetworkRxBytesTotal int64           `json:"network_rx_bytes_total"`
	NetworkTxBytesTotal int64           `json:"network_tx_bytes_total"`
}

func (s *Service) GetPlatformStats(ctx context.Context) (*PlatformStats, error) {
	containers, err := s.docker.ListAllContainers(ctx)
	if err != nil {
		return nil, err
	}

	known, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	byContainerID := make(map[string]*Server, len(known))
	for _, k := range known {
		if k.ContainerID != "" {
			byContainerID[k.ContainerID] = k
		}
	}

	running := containers[:0]
	for _, c := range containers {
		if c.State == "running" {
			running = append(running, c)
		}
	}

	results := make([]ContainerStat, len(running))
	var wg sync.WaitGroup
	for i, c := range running {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snapshot, err := s.docker.ContainerStats(ctx, c.ID)
			if err != nil {
				return
			}
			name := c.ID[:12]
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			cs := ContainerStat{
				ContainerID:    c.ID,
				Name:           name,
				Image:          c.Image,
				CPUPercent:     snapshot.CPUPercent,
				MemoryUsedMB:   snapshot.MemoryUsedMB,
				MemoryLimitMB:  snapshot.MemoryLimitMB,
				NetworkRxBytes: snapshot.NetworkRxBytes,
				NetworkTxBytes: snapshot.NetworkTxBytes,
			}
			if srv, ok := byContainerID[c.ID]; ok {
				cs.IsManaged = true
				cs.ServerID = srv.ID
				cs.ServerName = srv.Name
			}
			results[i] = cs
		}()
	}
	wg.Wait()

	stats := &PlatformStats{Containers: make([]ContainerStat, 0, len(results))}
	for _, cs := range results {
		if cs.ContainerID == "" {
			continue // falhou o fetch de stats desse container, pula
		}
		stats.Containers = append(stats.Containers, cs)
		stats.TotalCPUPercent += cs.CPUPercent
		stats.TotalMemoryUsedMB += cs.MemoryUsedMB
		stats.TotalMemoryLimitMB += cs.MemoryLimitMB
		stats.NetworkRxBytesTotal += cs.NetworkRxBytes
		stats.NetworkTxBytesTotal += cs.NetworkTxBytes
	}
	sort.Slice(stats.Containers, func(i, j int) bool {
		return stats.Containers[i].CPUPercent > stats.Containers[j].CPUPercent
	})

	du, err := s.docker.DiskUsage(ctx)
	if err == nil {
		stats.DiskUsedBytes = du.LayersSize
		for _, v := range du.Volumes {
			if v.UsageData != nil {
				stats.DiskUsedBytes += v.UsageData.Size
			}
		}
	}

	return stats, nil
}
