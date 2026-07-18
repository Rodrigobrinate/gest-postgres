package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

// CreateContainerInput descreve um container Postgres gerenciado a ser criado.
// Sobe com a config default da imagem — o subset de postgresql.conf do MVP é
// aplicado depois via ALTER SYSTEM (ver Service.applySettings), nunca por
// flag `-c` no comando: `-c` tem prioridade maior que ALTER SYSTEM e ficaria
// travado pra sempre, nem restart destravaria.
type CreateContainerInput struct {
	Name         string
	Image        string // ex: "postgres:16"
	Username     string
	Password     string
	DatabaseName string
	HostPort     int
	VolumeName   string
	NetworkName  string
	CPUCores     float64
	MemoryMB     int
	ServerID     string
}

type ContainerInfo struct {
	ID      string
	Status  string // running, exited, restarting, created, dead, paused
	Running bool
}

func (c *Client) CreateContainer(ctx context.Context, in CreateContainerInput) (string, error) {
	portBinding := nat.PortMap{
		"5432/tcp": []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", in.HostPort)},
		},
	}

	containerConfig := &container.Config{
		Image: in.Image,
		Env: []string{
			"POSTGRES_USER=" + in.Username,
			"POSTGRES_PASSWORD=" + in.Password,
			"POSTGRES_DB=" + in.DatabaseName,
		},
		ExposedPorts: nat.PortSet{"5432/tcp": struct{}{}},
		Labels: map[string]string{
			LabelManaged:  "true",
			LabelServerID: in.ServerID,
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBinding,
		Binds:        []string{in.VolumeName + ":/var/lib/postgresql/data"},
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		Resources: container.Resources{
			NanoCPUs: int64(in.CPUCores * 1e9),
			Memory:   int64(in.MemoryMB) * 1024 * 1024,
		},
	}

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: endpointsConfig(in.NetworkName),
	}

	created, err := c.cli.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, in.Name)
	if err != nil {
		return "", fmt.Errorf("criando container %s: %w", in.Name, err)
	}

	if err := c.cli.ContainerStart(ctx, created.ID, types.ContainerStartOptions{}); err != nil {
		return created.ID, fmt.Errorf("iniciando container %s: %w", in.Name, err)
	}

	return created.ID, nil
}

// UpdateContainerResources troca limite de CPU/memória de um container
// RODANDO sem recriar — o Docker suporta isso nativamente (diferente de
// porta publicada, que é fixada na criação). Não reinicia o Postgres nem
// derruba conexões.
func (c *Client) UpdateContainerResources(ctx context.Context, containerID string, cpuCores float64, memoryMB int) error {
	_, err := c.cli.ContainerUpdate(ctx, containerID, container.UpdateConfig{
		Resources: container.Resources{
			NanoCPUs: int64(cpuCores * 1e9),
			Memory:   int64(memoryMB) * 1024 * 1024,
		},
	})
	if err != nil {
		return fmt.Errorf("atualizando recursos do container %s: %w", containerID, err)
	}
	return nil
}

func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	if err := c.cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("iniciando container %s: %w", containerID, err)
	}
	return nil
}

func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	timeout := 30
	if err := c.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("parando container %s: %w", containerID, err)
	}
	return nil
}

func (c *Client) RestartContainer(ctx context.Context, containerID string) error {
	timeout := 30
	if err := c.cli.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("reiniciando container %s: %w", containerID, err)
	}
	return nil
}

// RemoveContainer remove o container. removeVolume também apaga o volume nomeado
// da instância — irreversível, a confirmação é responsabilidade da camada acima.
func (c *Client) RemoveContainer(ctx context.Context, containerID, volumeName string, removeVolume bool) error {
	if err := c.cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: true,
	}); err != nil {
		return fmt.Errorf("removendo container %s: %w", containerID, err)
	}

	if removeVolume && volumeName != "" {
		if err := c.cli.VolumeRemove(ctx, volumeName, true); err != nil {
			return fmt.Errorf("removendo volume %s: %w", volumeName, err)
		}
	}
	return nil
}

func (c *Client) InspectContainer(ctx context.Context, containerID string) (ContainerInfo, error) {
	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return ContainerInfo{}, fmt.Errorf("inspecionando container %s: %w", containerID, err)
	}
	return ContainerInfo{
		ID:      info.ID,
		Status:  info.State.Status,
		Running: info.State.Running,
	}, nil
}

// DiscoveryDetail junta o que a auto-descoberta precisa saber de um container
// que a plataforma não criou — em que redes ele já está, se tem porta 5432
// publicada, e se o data dir tá num volume nomeado (pra registrar de forma
// consistente com o resto do modelo, mesmo sem ter sido a plataforma que
// criou o volume).
type DiscoveryDetail struct {
	Name       string // sem a barra inicial que o Docker usa
	Image      string
	Networks   []string
	HostPort   int // porta publicada do 5432/tcp; 0 se não publicado
	VolumeName string
}

func (c *Client) InspectForDiscovery(ctx context.Context, containerID string) (DiscoveryDetail, error) {
	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return DiscoveryDetail{}, fmt.Errorf("inspecionando container %s: %w", containerID, err)
	}

	detail := DiscoveryDetail{
		Name:  strings.TrimPrefix(info.Name, "/"),
		Image: info.Config.Image,
	}
	if info.NetworkSettings != nil {
		for netName := range info.NetworkSettings.Networks {
			detail.Networks = append(detail.Networks, netName)
		}
	}
	if info.HostConfig != nil {
		if bindings, ok := info.HostConfig.PortBindings["5432/tcp"]; ok && len(bindings) > 0 {
			fmt.Sscanf(bindings[0].HostPort, "%d", &detail.HostPort)
		}
	}
	for _, m := range info.Mounts {
		if m.Destination == "/var/lib/postgresql/data" && m.Type == "volume" {
			detail.VolumeName = m.Name
			break
		}
	}
	return detail, nil
}

// ListManagedContainers retorna todos os containers com a label gestpg.managed=true,
// usado pra reconciliar estado na inicialização (containers que existem no Docker
// mas cujo status no metadata DB ficou desatualizado).
func (c *Client) ListManagedContainers(ctx context.Context) ([]types.Container, error) {
	containers, err := c.cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", LabelManaged+"=true")),
	})
	if err != nil {
		return nil, fmt.Errorf("listando containers gerenciados: %w", err)
	}
	return containers, nil
}

// ListAllContainers lista TODOS os containers do host (não só os gerenciados
// pela plataforma) — usado só pela auto-descoberta, que precisa enxergar o
// que já existe fora do nosso controle pra sugerir cadastro.
func (c *Client) ListAllContainers(ctx context.Context) ([]types.Container, error) {
	containers, err := c.cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("listando containers: %w", err)
	}
	return containers, nil
}

// ConnectNetwork anexa um container já existente (não criado por nós) à rede
// gerenciada — necessário pra conseguir falar com ele pelo nome depois de
// "adotado" via auto-descoberta. Idempotente: já conectado não é erro.
func (c *Client) ConnectNetwork(ctx context.Context, networkName, containerID string) error {
	err := c.cli.NetworkConnect(ctx, networkName, containerID, nil)
	if err != nil && !isAlreadyConnectedErr(err) {
		return fmt.Errorf("conectando container %s à rede %s: %w", containerID, networkName, err)
	}
	return nil
}

func isAlreadyConnectedErr(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "already exists in network") || strings.Contains(err.Error(), "already attached"))
}

// WaitHealthy faz polling simples do status até o container reportar "running",
// com timeout. Não usa healthcheck do Docker (a imagem postgres oficial não define
// um por padrão) — a checagem real de "aceita conexões" fica pro service, que tenta
// abrir uma conexão pgx com retry.
func (c *Client) WaitHealthy(ctx context.Context, containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		info, err := c.InspectContainer(ctx, containerID)
		if err != nil {
			return err
		}
		if info.Running {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("timeout esperando container %s ficar running", containerID)
}
