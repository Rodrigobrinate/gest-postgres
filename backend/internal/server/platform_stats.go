package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/gest-postgres/backend/internal/docker"
)

type ContainerStat struct {
	ContainerID     string  `json:"container_id"`
	Name            string  `json:"name"`
	Image           string  `json:"image"`
	IsManaged       bool    `json:"is_managed"`
	Adoptable       bool    `json:"adoptable"` // parece Postgres, não gerenciado ainda, e não é container do próprio stack — dá pra clicar e cadastrar
	ServerID        string  `json:"server_id,omitempty"`
	ServerName      string  `json:"server_name,omitempty"`
	CPUPercent      float64 `json:"cpu_percent"`
	MemoryUsedMB    float64 `json:"memory_used_mb"`
	MemoryLimitMB   float64 `json:"memory_limit_mb"`
	NetworkRxBytes  int64   `json:"network_rx_bytes"`
	NetworkTxBytes  int64   `json:"network_tx_bytes"`
	BlockReadBytes  int64   `json:"block_read_bytes"`
	BlockWriteBytes int64   `json:"block_write_bytes"`
	BlockReadOps    int64   `json:"block_read_ops"`
	BlockWriteOps   int64   `json:"block_write_ops"`
	VolumeSizeBytes int64   `json:"volume_size_bytes,omitempty"` // "peso" do container: soma dos volumes nomeados montados, ou (sem volume) a camada gravável
}

// PlatformStats agrega TODOS os containers Docker do host (não só os
// gerenciados) — é o proxy honesto de "recursos da plataforma" que dá pra
// ter sem acesso ao host além da API Docker. Disco é exceção: vem de
// statfs real no host via mount /hostfs (ver docker-compose.yml e
// docker.HostDiskUsage) — os outros (CPU/memória/rede/I/O) continuam sendo
// soma dos containers, não tem como pegar "de fora do Docker" sem isso.
type PlatformStats struct {
	Containers          []ContainerStat `json:"containers"`
	TotalCPUPercent     float64         `json:"total_cpu_percent"`
	TotalMemoryUsedMB   float64         `json:"total_memory_used_mb"`
	TotalMemoryLimitMB  float64         `json:"total_memory_limit_mb"`
	DiskTotalBytes      int64           `json:"disk_total_bytes"`
	DiskUsedBytes       int64           `json:"disk_used_bytes"`
	DiskFreeBytes       int64           `json:"disk_free_bytes"`
	DiskAvailable       bool            `json:"disk_available"`         // false se o mount /hostfs não existe
	DockerDiskUsedBytes int64           `json:"docker_disk_used_bytes"` // subconjunto: só imagens+containers+volumes
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

	// Busca de uma vez só, fora do loop de containers — dá o agregado
	// (imagens+containers+volumes), o tamanho de cada volume nomeado
	// individualmente, e a camada gravável (SizeRw) de cada container. Isso
	// cobre "peso" pra QUALQUER container, não só Postgres gerenciado: quem
	// tem volume nomeado usa a soma dos volumes montados; quem não tem (ex:
	// container sem persistência dedicada) cai pro tamanho da própria
	// camada gravável, que é o proxy honesto de "quanto esse container
	// específico gravou" sem contar as camadas de imagem compartilhadas
	// (SizeRootFs infla isso porque conta a imagem base inteira).
	du, duErr := s.docker.DiskUsage(ctx)
	volumeSizes := make(map[string]int64)
	containerRwSizes := make(map[string]int64)
	if duErr == nil {
		for _, v := range du.Volumes {
			if v.UsageData != nil {
				volumeSizes[v.Name] = v.UsageData.Size
			}
		}
		for _, dc := range du.Containers {
			containerRwSizes[dc.ID] = dc.SizeRw
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
				ContainerID:     c.ID,
				Name:            name,
				Image:           c.Image,
				CPUPercent:      snapshot.CPUPercent,
				MemoryUsedMB:    snapshot.MemoryUsedMB,
				MemoryLimitMB:   snapshot.MemoryLimitMB,
				NetworkRxBytes:  snapshot.NetworkRxBytes,
				NetworkTxBytes:  snapshot.NetworkTxBytes,
				BlockReadBytes:  snapshot.BlockReadBytes,
				BlockWriteBytes: snapshot.BlockWriteBytes,
				BlockReadOps:    snapshot.BlockReadOps,
				BlockWriteOps:   snapshot.BlockWriteOps,
			}
			if srv, ok := byContainerID[c.ID]; ok {
				cs.IsManaged = true
				cs.ServerID = srv.ID
				cs.ServerName = srv.Name
			}

			if !cs.IsManaged && !ownComposeServices[c.Labels["com.docker.compose.service"]] {
				ports := make([]string, 0, len(c.Ports))
				for _, p := range c.Ports {
					if p.PublicPort > 0 {
						ports = append(ports, fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type))
					} else {
						ports = append(ports, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
					}
				}
				cs.Adoptable = looksLikePostgres(c.Image, ports)
			}

			var volTotal int64
			var hasVolume bool
			for _, m := range c.Mounts {
				if m.Type == "volume" && m.Name != "" {
					if sz, ok := volumeSizes[m.Name]; ok {
						volTotal += sz
						hasVolume = true
					}
				}
			}
			if hasVolume {
				cs.VolumeSizeBytes = volTotal
			} else if sz, ok := containerRwSizes[c.ID]; ok {
				cs.VolumeSizeBytes = sz
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
		stats.NetworkRxBytesTotal += cs.NetworkRxBytes
		stats.NetworkTxBytesTotal += cs.NetworkTxBytes
	}

	// Memória TOTAL e USADA vêm de /proc/meminfo do HOST de verdade (mount
	// /hostmem, ver docker.HostMemoryUsage), nunca de somar
	// MemoryLimitMB/MemoryUsedMB por container. Pro total, isso já era
	// assim antes por outro motivo (container sem limite explícito reporta
	// o host inteiro como "limite", e somar isso entre vários containers
	// sem limite infla o total pra um múltiplo do real — bug visto ao vivo:
	// dashboard mostrando "de 14665 MB" numa máquina de ~1.9GB). Pro
	// usado, o motivo é outro: soma de container nunca inclui
	// kernel/dockerd/sshd/cron/firewall-agent/update-agent (tudo fora de
	// cgroup Docker) — comparado ao vivo contra o Painel do EasyPanel no
	// mesmo host, que lê o host de verdade e mostrava bem mais memória em
	// uso. Cada container individual continua mostrando o limite/uso dele
	// próprio (cs.MemoryLimitMB/cs.MemoryUsedMB) — só o AGREGADO da
	// plataforma troca de fonte, mesmo raciocínio já usado pro disco
	// (docker.HostDiskUsage). Sem esse mount (instalação antiga que ainda
	// não passou pelo setup.sh dessa versão), cai de volta pra soma de
	// container, silenciosamente.
	if memTotal, memUsed, err := docker.HostMemoryUsage(); err == nil {
		stats.TotalMemoryLimitMB = float64(memTotal) / (1024 * 1024)
		stats.TotalMemoryUsedMB = float64(memUsed) / (1024 * 1024)
	}

	// CPU do host de verdade via /proc/stat (mount /hostcpu) — mesmo
	// raciocínio da memória acima. Primeira leitura depois do backend
	// subir sempre erra (sem amostra anterior pra comparar, ver
	// docker.HostCPUPercent) — nesse caso mantém a soma de container já
	// calculada acima em vez de zerar.
	if cpuPercent, err := docker.HostCPUPercent(); err == nil {
		stats.TotalCPUPercent = cpuPercent
	}
	sort.Slice(stats.Containers, func(i, j int) bool {
		return stats.Containers[i].CPUPercent > stats.Containers[j].CPUPercent
	})

	if duErr == nil {
		stats.DockerDiskUsedBytes = du.LayersSize
		for _, size := range volumeSizes {
			stats.DockerDiskUsedBytes += size
		}
	}

	if total, used, free, err := docker.HostDiskUsage(); err == nil {
		stats.DiskAvailable = true
		stats.DiskTotalBytes = total
		stats.DiskUsedBytes = used
		stats.DiskFreeBytes = free
	}

	return stats, nil
}
