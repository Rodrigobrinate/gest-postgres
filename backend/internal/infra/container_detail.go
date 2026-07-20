package infra

import (
	"context"
	"fmt"
	"strings"

	"github.com/gest-postgres/backend/internal/docker"
)

func (s *Service) ContainerDetail(ctx context.Context, id string) (docker.ContainerDetail, error) {
	return s.docker.InspectContainerFull(ctx, id)
}

func (s *Service) ContainerStats(ctx context.Context, id string) (docker.ContainerStatsSnapshot, error) {
	return s.docker.ContainerStats(ctx, id)
}

// UpdateContainerResources troca CPU/memória de um container RODANDO sem
// recriar — mesmo mecanismo já usado pra servidor Postgres gerenciado
// (ver server.Service), agora exposto pra container genérico também.
func (s *Service) UpdateContainerResources(ctx context.Context, containerID string, cpuCores float64, memoryMB int) error {
	return s.docker.UpdateContainerResources(ctx, containerID, cpuCores, memoryMB)
}

func (s *Service) ConnectContainerNetwork(ctx context.Context, containerID, networkName string) error {
	return s.docker.ConnectNetwork(ctx, networkName, containerID)
}

func (s *Service) DisconnectContainerNetwork(ctx context.Context, containerID, networkName string) error {
	return s.docker.DisconnectNetwork(ctx, networkName, containerID, false)
}

// UpdateContainerEnv recria o container com variáveis de ambiente novas —
// Docker não suporta trocar env var de container rodando, só recriando
// (mesma limitação/mecanismo do anexar volume). O ID do container MUDA
// depois dessa chamada.
func (s *Service) UpdateContainerEnv(ctx context.Context, containerID string, env map[string]string) (string, error) {
	envList := make([]string, 0, len(env))
	for k, v := range env {
		if k == "" {
			continue
		}
		envList = append(envList, k+"="+v)
	}
	return s.docker.RecreateContainerWithEnv(ctx, containerID, envList)
}

type AttachVolumeInput struct {
	VolumeName string `json:"volume_name"`
	MountPath  string `json:"mount_path"`
	ReadOnly   bool   `json:"read_only"`
}

// AttachVolumeToContainer recria o container com um bind novo — Docker não
// suporta anexar volume a um container já existente sem recriar (ver
// docker.RecreateContainerWithExtraBind). O ID do container MUDA depois
// dessa chamada; quem chama precisa navegar pro novo ID.
func (s *Service) AttachVolumeToContainer(ctx context.Context, containerID string, in AttachVolumeInput) (string, error) {
	if in.VolumeName == "" || in.MountPath == "" {
		return "", fmt.Errorf("volume e caminho de montagem são obrigatórios")
	}
	if !strings.HasPrefix(in.MountPath, "/") {
		return "", fmt.Errorf("caminho de montagem deve ser absoluto")
	}
	bind := in.VolumeName + ":" + in.MountPath
	if in.ReadOnly {
		bind += ":ro"
	}
	return s.docker.RecreateContainerWithExtraBind(ctx, containerID, bind)
}
