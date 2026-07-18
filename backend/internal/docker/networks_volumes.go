package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/go-connections/nat"
)

type NetworkSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`
}

func (c *Client) ListNetworks(ctx context.Context) ([]NetworkSummary, error) {
	nets, err := c.cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listando redes: %w", err)
	}
	out := make([]NetworkSummary, 0, len(nets))
	for _, n := range nets {
		out = append(out, NetworkSummary{ID: n.ID, Name: n.Name, Driver: n.Driver, Scope: n.Scope})
	}
	return out, nil
}

// CreateNetwork cria uma rede bridge nova (uso genérico — não confundir com
// EnsureNetwork, que é só pras redes internas fixas da plataforma).
func (c *Client) CreateNetwork(ctx context.Context, name string) (string, error) {
	created, err := c.cli.NetworkCreate(ctx, name, types.NetworkCreate{Driver: "bridge"})
	if err != nil {
		return "", fmt.Errorf("criando rede %s: %w", name, err)
	}
	return created.ID, nil
}

func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	if err := c.cli.NetworkRemove(ctx, id); err != nil {
		return fmt.Errorf("removendo rede %s: %w", id, err)
	}
	return nil
}

type VolumeSummary struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Mountpoint string `json:"mountpoint"`
	SizeBytes  int64  `json:"size_bytes,omitempty"`
}

func (c *Client) ListVolumes(ctx context.Context) ([]VolumeSummary, error) {
	resp, err := c.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listando volumes: %w", err)
	}
	out := make([]VolumeSummary, 0, len(resp.Volumes))
	for _, v := range resp.Volumes {
		vs := VolumeSummary{Name: v.Name, Driver: v.Driver, Mountpoint: v.Mountpoint}
		if v.UsageData != nil {
			vs.SizeBytes = v.UsageData.Size
		}
		out = append(out, vs)
	}
	return out, nil
}

func (c *Client) CreateVolume(ctx context.Context, name string) error {
	_, err := c.cli.VolumeCreate(ctx, volume.CreateOptions{Name: name})
	if err != nil {
		return fmt.Errorf("criando volume %s: %w", name, err)
	}
	return nil
}

func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	if err := c.cli.VolumeRemove(ctx, name, force); err != nil {
		return fmt.Errorf("removendo volume %s: %w", name, err)
	}
	return nil
}

// CreateGenericContainerInput é o irmão mais flexível de CreateContainerInput
// — usado por container genérico/Traefik, onde as portas/binds/comando
// variam por caso, diferente de "sempre 5432 + volume de data" do Postgres.
type CreateGenericContainerInput struct {
	Name                 string
	Image                string
	Command              []string
	Env                  []string
	Ports                map[string]string // "containerPort/tcp" -> "hostPort" (hostPort "" = não publica)
	Binds                []string          // "volumeOrHostPath:containerPath[:ro]"
	NetworkName          string
	Labels               map[string]string
	RestartUnlessStopped bool
}

// CreateGenericContainer é o irmão flexível de CreateContainer — usado por
// Traefik e por containers genéricos criados pela tela de "Docker", onde
// porta/bind/comando variam por caso, diferente do Postgres (sempre 5432 +
// um volume de data).
func (c *Client) CreateGenericContainer(ctx context.Context, in CreateGenericContainerInput) (string, error) {
	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}
	for containerPort, hostPort := range in.Ports {
		port := nat.Port(containerPort)
		exposedPorts[port] = struct{}{}
		if hostPort != "" {
			portBindings[port] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hostPort}}
		}
	}

	restartPolicy := container.RestartPolicy{}
	if in.RestartUnlessStopped {
		restartPolicy.Name = "unless-stopped"
	}

	containerConfig := &container.Config{
		Image:        in.Image,
		Cmd:          in.Command,
		Env:          in.Env,
		ExposedPorts: exposedPorts,
		Labels:       in.Labels,
	}
	hostConfig := &container.HostConfig{
		PortBindings:  portBindings,
		Binds:         in.Binds,
		RestartPolicy: restartPolicy,
	}
	var networkingConfig *network.NetworkingConfig
	if in.NetworkName != "" {
		networkingConfig = &network.NetworkingConfig{EndpointsConfig: endpointsConfig(in.NetworkName)}
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
