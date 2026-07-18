package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
)

// ContainerLogs busca as últimas tailLines linhas de stdout+stderr do
// container. A imagem postgres oficial não roda com TTY, então o stream vem
// multiplexado (formato Docker) e precisa ser demultiplexado com stdcopy.
func (c *Client) ContainerLogs(ctx context.Context, containerID string, tailLines int) (string, error) {
	return c.containerLogs(ctx, containerID, tailLines, false)
}

// ContainerLogsWithTimestamps é igual, mas pede pro Docker prefixar cada
// linha com um timestamp RFC3339Nano — não depende do log_line_prefix do
// Postgres (que pode nem ter timestamp configurado), então dá pra cruzar
// qualquer log com o histórico de métricas de forma confiável.
func (c *Client) ContainerLogsWithTimestamps(ctx context.Context, containerID string, tailLines int) (string, error) {
	return c.containerLogs(ctx, containerID, tailLines, true)
}

func (c *Client) containerLogs(ctx context.Context, containerID string, tailLines int, timestamps bool) (string, error) {
	reader, err := c.cli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       strconv.Itoa(tailLines),
		Timestamps: timestamps,
	})
	if err != nil {
		return "", fmt.Errorf("lendo logs do container %s: %w", containerID, err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, &buf, reader); err != nil {
		return "", fmt.Errorf("processando stream de logs: %w", err)
	}
	return buf.String(), nil
}

type ContainerStatsSnapshot struct {
	CPUPercent      float64 `json:"cpu_percent"`
	MemoryUsedMB    float64 `json:"memory_used_mb"`
	MemoryLimitMB   float64 `json:"memory_limit_mb"`
	MemoryPercent   float64 `json:"memory_percent"`
	NetworkRxBytes  int64   `json:"network_rx_bytes"` // cumulativo desde que o container subiu, não taxa
	NetworkTxBytes  int64   `json:"network_tx_bytes"`
}

// dockerStatsRaw só declara os campos que a gente usa — o JSON completo de
// /containers/{id}/stats tem muito mais coisa (block I/O, rede, etc), fora de
// escopo do MVP.
type dockerStatsRaw struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
		Stats struct {
			Cache uint64 `json:"cache"`
		} `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

// ContainerStats pega um snapshot único (sem stream) de CPU/memória, com o
// mesmo cálculo de CPU% que o `docker stats` usa (delta de uso / delta do
// sistema * cores online).
func (c *Client) ContainerStats(ctx context.Context, containerID string) (ContainerStatsSnapshot, error) {
	resp, err := c.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return ContainerStatsSnapshot{}, fmt.Errorf("lendo stats do container %s: %w", containerID, err)
	}
	defer resp.Body.Close()

	var raw dockerStatsRaw
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return ContainerStatsSnapshot{}, fmt.Errorf("decodificando stats do container %s: %w", containerID, err)
	}

	var cpuPercent float64
	cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage) - float64(raw.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(raw.CPUStats.SystemCPUUsage) - float64(raw.PreCPUStats.SystemCPUUsage)
	if systemDelta > 0 && cpuDelta > 0 {
		onlineCPUs := float64(raw.CPUStats.OnlineCPUs)
		if onlineCPUs == 0 {
			onlineCPUs = 1
		}
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	memUsage := float64(raw.MemoryStats.Usage) - float64(raw.MemoryStats.Stats.Cache)
	if memUsage < 0 {
		memUsage = float64(raw.MemoryStats.Usage)
	}
	memLimit := float64(raw.MemoryStats.Limit)
	var memPercent float64
	if memLimit > 0 {
		memPercent = (memUsage / memLimit) * 100.0
	}

	var rxBytes, txBytes uint64
	for _, n := range raw.Networks {
		rxBytes += n.RxBytes
		txBytes += n.TxBytes
	}

	return ContainerStatsSnapshot{
		CPUPercent:     cpuPercent,
		MemoryUsedMB:   memUsage / 1024 / 1024,
		MemoryLimitMB:  memLimit / 1024 / 1024,
		MemoryPercent:  memPercent,
		NetworkRxBytes: int64(rxBytes),
		NetworkTxBytes: int64(txBytes),
	}, nil
}
