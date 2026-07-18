package infra

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gest-postgres/backend/internal/docker"
)

type ContainerSummary struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Image    string            `json:"image"`
	State    string            `json:"state"`
	Status   string            `json:"status"`
	Ports    []string          `json:"ports"`
	Networks []string          `json:"networks"`
	Labels   map[string]string `json:"labels"`
	Project  string            `json:"project,omitempty"` // com.docker.compose.project, se aplicável
}

func (s *Service) ListContainers(ctx context.Context) ([]ContainerSummary, error) {
	all, err := s.docker.ListAllContainers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ContainerSummary, 0, len(all))
	for _, c := range all {
		name := c.ID[:12]
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		ports := make([]string, 0, len(c.Ports))
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type))
			} else {
				ports = append(ports, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
			}
		}
		var nets []string
		if c.NetworkSettings != nil {
			for netName := range c.NetworkSettings.Networks {
				nets = append(nets, netName)
			}
		}
		out = append(out, ContainerSummary{
			ID:       c.ID,
			Name:     name,
			Image:    c.Image,
			State:    c.State,
			Status:   c.Status,
			Ports:    ports,
			Networks: nets,
			Labels:   c.Labels,
			Project:  c.Labels["com.docker.compose.project"],
		})
	}
	return out, nil
}

func (s *Service) StartContainer(ctx context.Context, id string) error {
	return s.docker.StartContainer(ctx, id)
}

func (s *Service) StopContainer(ctx context.Context, id string) error {
	return s.docker.StopContainer(ctx, id)
}

func (s *Service) RestartContainer(ctx context.Context, id string) error {
	return s.docker.RestartContainer(ctx, id)
}

// RemoveContainer nunca apaga volume junto — isso é uma decisão separada,
// tomada explicitamente na tela de volumes (evita apagar dado por engano só
// por ter removido o container que o usava).
func (s *Service) RemoveContainer(ctx context.Context, id string) error {
	return s.docker.RemoveContainer(ctx, id, "", false)
}

func (s *Service) ContainerLogs(ctx context.Context, id string, tailLines int) (string, error) {
	return s.docker.ContainerLogsWithTimestamps(ctx, id, tailLines)
}

type CreateContainerFromImageInput struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Env         map[string]string `json:"env"`
	Ports       map[string]int    `json:"ports"` // "containerPort/tcp" -> hostPort (0 = não publica)
	NetworkName string            `json:"network"`
}

// CreateContainerFromImage é o "criar container" simples da tela de Docker —
// puxa a imagem se precisar e sobe. Pra casos mais elaborados (múltiplos
// serviços, volumes nomeados, dependências) a rota é compose, não essa.
func (s *Service) CreateContainerFromImage(ctx context.Context, in CreateContainerFromImageInput) (string, error) {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Image) == "" {
		return "", fmt.Errorf("nome e imagem são obrigatórios")
	}
	if err := s.docker.PullImageIfMissing(ctx, in.Image); err != nil {
		return "", err
	}

	env := make([]string, 0, len(in.Env))
	for k, v := range in.Env {
		env = append(env, k+"="+v)
	}

	ports := make(map[string]string, len(in.Ports))
	for containerPort, hostPort := range in.Ports {
		if hostPort > 0 {
			ports[containerPort] = strconv.Itoa(hostPort)
		} else {
			ports[containerPort] = ""
		}
	}

	networkName := in.NetworkName
	if networkName == "" {
		networkName = s.networkName
	}

	return s.docker.CreateGenericContainer(ctx, docker.CreateGenericContainerInput{
		Name:                 in.Name,
		Image:                in.Image,
		Env:                  env,
		Ports:                ports,
		NetworkName:          networkName,
		RestartUnlessStopped: true,
	})
}
