package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
)

// RecreateContainerWithExtraBind para, remove (sem apagar volume nenhum) e
// recria um container com a MESMA imagem/env/comando/labels/portas/restart
// policy que ele já tinha, mais um bind novo (volume montado num path). É a
// única forma de anexar volume a um container já existente — o Docker não
// deixa fazer isso ao vivo.
//
// Best-effort de propósito: reconecta nas mesmas redes que já estava
// (por nome, sem preservar IP fixo/aliases customizados), mas não lê nem
// recria configuração mais avançada que o HostConfig original não carregue
// 1:1 pro ContainerCreate (ex: alguns campos calculados). Pra um container
// que a própria plataforma criou (via imagem/Dockerfile/Git) isso cobre
// bem; pra um container "adotado" de fora, o HostConfig devolvido pelo
// inspect já é o reflexo fiel do que existe hoje, então normalmente ainda
// funciona — mas sem garantia formal, por isso a UI avisa a diferença
// (ver frontend, aba Volumes do detalhe do container).
func (c *Client) RecreateContainerWithExtraBind(ctx context.Context, containerID, extraBind string) (string, error) {
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
	hostConfig.Binds = append(append([]string{}, hostConfig.Binds...), extraBind)

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
