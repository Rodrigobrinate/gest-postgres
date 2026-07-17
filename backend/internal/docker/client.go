// Package docker encapsula toda comunicação com o Docker Engine API usada pra
// provisionar os Postgres gerenciados. Nunca fala com o socket direto — DOCKER_HOST
// deve apontar pro docker-socket-proxy (ver docker-compose.yml raiz).
package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// Labels usadas pra identificar e filtrar containers criados pela plataforma.
const (
	LabelManaged  = "gestpg.managed"
	LabelServerID = "gestpg.server_id"
)

type Client struct {
	cli *client.Client
}

func NewClient(host string) (*Client, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("criando client docker: %w", err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}

// EnsureNetwork garante que a rede compartilhada dos servidores gerenciados existe.
func (c *Client) EnsureNetwork(ctx context.Context, name string) error {
	nets, err := c.cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(filters.Arg("name", name)),
	})
	if err != nil {
		return fmt.Errorf("listando redes: %w", err)
	}
	for _, n := range nets {
		if n.Name == name {
			return nil
		}
	}
	_, err = c.cli.NetworkCreate(ctx, name, types.NetworkCreate{
		Driver: "bridge",
		Options: map[string]string{
			"com.docker.network.bridge.enable_icc": "true",
		},
	})
	if err != nil {
		return fmt.Errorf("criando rede %s: %w", name, err)
	}
	return nil
}

// EnsureVolume cria o volume nomeado da instância se ainda não existir.
func (c *Client) EnsureVolume(ctx context.Context, name string) error {
	_, err := c.cli.VolumeInspect(ctx, name)
	if err == nil {
		return nil
	}
	_, err = c.cli.VolumeCreate(ctx, volume.CreateOptions{Name: name})
	if err != nil {
		return fmt.Errorf("criando volume %s: %w", name, err)
	}
	return nil
}

// PullImageIfMissing baixa a imagem se ela ainda não existir localmente.
func (c *Client) PullImageIfMissing(ctx context.Context, image string) error {
	_, _, err := c.cli.ImageInspectWithRaw(ctx, image)
	if err == nil {
		return nil
	}

	reader, err := c.cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("iniciando pull da imagem %s: %w", image, err)
	}
	defer reader.Close()

	// Precisa drenar o stream até o fim, senão o pull não completa.
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("baixando imagem %s: %w", image, err)
	}
	return nil
}

func endpointsConfig(networkName string) map[string]*network.EndpointSettings {
	return map[string]*network.EndpointSettings{
		networkName: {},
	}
}
