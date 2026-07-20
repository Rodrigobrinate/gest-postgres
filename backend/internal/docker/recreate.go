package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// recreateContainer é o miolo compartilhado: para, remove (sem apagar
// volume nenhum), deixa `mutate` ajustar a config original antes de recriar
// com o mesmo nome, reconecta nas mesmas redes que já estava e inicia de
// novo. É a base de qualquer operação que precisa mudar algo que o Docker
// só aceita na criação (variável de ambiente, bind de volume — nenhum dos
// dois dá pra alterar num container já rodando).
//
// Best-effort de propósito: reconecta nas mesmas redes que já estava (por
// nome, sem preservar IP fixo/aliases customizados), mas não lê nem recria
// configuração mais avançada que o HostConfig original não carregue 1:1 pro
// ContainerCreate (ex: alguns campos calculados). Pra um container que a
// própria plataforma criou (via imagem/Dockerfile/Git) isso cobre bem; pra
// um container "adotado" de fora, o HostConfig devolvido pelo inspect já é
// o reflexo fiel do que existe hoje, então normalmente ainda funciona —
// mas sem garantia formal, por isso a UI avisa a diferença.
func (c *Client) recreateContainer(ctx context.Context, containerID string, mutate func(cfg *container.Config, hostCfg *container.HostConfig)) (string, error) {
	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("inspecionando container %s: %w", containerID, err)
	}
	name := strings.TrimPrefix(info.Name, "/")

	if err := c.StopContainer(ctx, containerID); err != nil {
		return "", err
	}
	if err := c.cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true}); err != nil {
		return "", fmt.Errorf("removendo container %s pra recriar: %w", containerID, err)
	}

	hostConfig := info.HostConfig
	if hostConfig == nil {
		return "", fmt.Errorf("container %s sem HostConfig — não dá pra recriar", containerID)
	}
	mutate(info.Config, hostConfig)

	created, err := c.cli.ContainerCreate(ctx, info.Config, hostConfig, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("recriando container %s: %w", name, err)
	}

	if info.NetworkSettings != nil {
		for netName := range info.NetworkSettings.Networks {
			if err := c.ConnectNetwork(ctx, netName, created.ID); err != nil {
				return created.ID, fmt.Errorf("reconectando rede %s no container recriado: %w", netName, err)
			}
		}
	}

	if err := c.StartContainer(ctx, created.ID); err != nil {
		return created.ID, err
	}
	return created.ID, nil
}

// RecreateContainerWithExtraBind anexa um volume novo — é a única forma de
// fazer isso num container já existente (Docker não suporta ao vivo).
func (c *Client) RecreateContainerWithExtraBind(ctx context.Context, containerID, extraBind string) (string, error) {
	return c.recreateContainer(ctx, containerID, func(_ *container.Config, hostCfg *container.HostConfig) {
		hostCfg.Binds = append(append([]string{}, hostCfg.Binds...), extraBind)
	})
}

// RecreateContainerWithEnv troca a lista de variáveis de ambiente inteira —
// mesma limitação do Docker (env var é fixada na criação, não existe
// "update" ao vivo, diferente de recursos CPU/memória que o Docker
// suporta trocar sem recriar).
func (c *Client) RecreateContainerWithEnv(ctx context.Context, containerID string, env []string) (string, error) {
	return c.recreateContainer(ctx, containerID, func(cfg *container.Config, _ *container.HostConfig) {
		cfg.Env = env
	})
}
