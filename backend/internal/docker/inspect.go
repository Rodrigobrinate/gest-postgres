package docker

import (
	"context"
	"fmt"
	"strings"
)

type MountInfo struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"` // só preenchido pra mount tipo "volume"
	RW          bool   `json:"rw"`
}

type NetworkEndpoint struct {
	IPAddress  string `json:"ip_address"`
	Gateway    string `json:"gateway"`
	MacAddress string `json:"mac_address"`
}

// ContainerDetail é a versão rica de ContainerInfo — usada só pela tela de
// detalhe de um container genérico (aba Docker), não pelo ciclo de vida de
// servidor Postgres gerenciado, que já tem seu próprio modelo mais
// específico (ver internal/server).
type ContainerDetail struct {
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Image         string                     `json:"image"`
	Status        string                     `json:"status"`
	Running       bool                       `json:"running"`
	CreatedAt     string                     `json:"created_at"`
	StartedAt     string                     `json:"started_at,omitempty"`
	FinishedAt    string                     `json:"finished_at,omitempty"`
	ExitCode      int                        `json:"exit_code"`
	RestartPolicy string                     `json:"restart_policy"`
	Labels        map[string]string          `json:"labels"`
	Env           []string                   `json:"env"`
	Command       []string                   `json:"command"`
	Mounts        []MountInfo                `json:"mounts"`
	Networks      map[string]NetworkEndpoint `json:"networks"`
	// CPUCores/MemoryMB: 0 = sem limite configurado (Docker default).
	CPUCores float64 `json:"cpu_cores"`
	MemoryMB int64   `json:"memory_mb"`
}

func (c *Client) InspectContainerFull(ctx context.Context, containerID string) (ContainerDetail, error) {
	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return ContainerDetail{}, fmt.Errorf("inspecionando container %s: %w", containerID, err)
	}

	detail := ContainerDetail{
		ID:      info.ID,
		Name:    strings.TrimPrefix(info.Name, "/"),
		Image:   info.Config.Image,
		Labels:  info.Config.Labels,
		Command: info.Config.Cmd,
		Mounts:  make([]MountInfo, 0, len(info.Mounts)),
	}
	if info.State != nil {
		detail.Status = info.State.Status
		detail.Running = info.State.Running
		detail.StartedAt = info.State.StartedAt
		detail.FinishedAt = info.State.FinishedAt
		detail.ExitCode = info.State.ExitCode
	}
	if info.Created != "" {
		detail.CreatedAt = info.Created
	}
	if info.Config != nil {
		detail.Env = info.Config.Env
	}
	if info.HostConfig != nil {
		detail.RestartPolicy = info.HostConfig.RestartPolicy.Name
		if info.HostConfig.NanoCPUs > 0 {
			detail.CPUCores = float64(info.HostConfig.NanoCPUs) / 1e9
		}
		if info.HostConfig.Memory > 0 {
			detail.MemoryMB = info.HostConfig.Memory / 1024 / 1024
		}
	}
	for _, m := range info.Mounts {
		detail.Mounts = append(detail.Mounts, MountInfo{
			Source:      m.Source,
			Destination: m.Destination,
			Type:        string(m.Type),
			Name:        m.Name,
			RW:          m.RW,
		})
	}
	if info.NetworkSettings != nil {
		detail.Networks = make(map[string]NetworkEndpoint, len(info.NetworkSettings.Networks))
		for name, ep := range info.NetworkSettings.Networks {
			if ep == nil {
				continue
			}
			detail.Networks[name] = NetworkEndpoint{
				IPAddress:  ep.IPAddress,
				Gateway:    ep.Gateway,
				MacAddress: ep.MacAddress,
			}
		}
	}
	return detail, nil
}

// DisconnectNetwork desconecta um container de uma rede — espelha
// ConnectNetwork. force=true derruba mesmo se o Docker achar que ainda tem
// endpoint ativo (caso raro de rede em estado inconsistente).
func (c *Client) DisconnectNetwork(ctx context.Context, networkName, containerID string, force bool) error {
	if err := c.cli.NetworkDisconnect(ctx, networkName, containerID, force); err != nil {
		return fmt.Errorf("desconectando container %s da rede %s: %w", containerID, networkName, err)
	}
	return nil
}
