package docker

import (
	"bytes"
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
)

// defaultShellCmd tenta bash primeiro (mais confortável — histórico, edição
// de linha) e cai pra sh se a imagem não tiver bash (ex: alpine sem
// bash instalado). Uma imagem sem os dois nem teria como rodar um shell de
// qualquer jeito.
var defaultShellCmd = []string{"sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi"}

// ExecCreate abre um processo de exec interativo (Tty) dentro do container —
// é o mesmo mecanismo do `docker exec -it`. Requer a categoria EXEC ligada
// no docker-socket-proxy (ver docker-compose.yml raiz, decisão consciente,
// ver CLAUDE.md).
func (c *Client) ExecCreate(ctx context.Context, containerID string, cmd []string) (string, error) {
	if len(cmd) == 0 {
		cmd = defaultShellCmd
	}
	resp, err := c.cli.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		return "", fmt.Errorf("criando exec no container %s: %w", containerID, err)
	}
	return resp.ID, nil
}

// ExecAttach conecta no processo de exec já criado — a conexão hijacked
// devolvida carrega stdin/stdout/stderr combinados num único stream (Tty
// sempre mescla os dois, não precisa de stdcopy aqui).
func (c *Client) ExecAttach(ctx context.Context, execID string) (types.HijackedResponse, error) {
	resp, err := c.cli.ContainerExecAttach(ctx, execID, types.ExecStartCheck{Tty: true})
	if err != nil {
		return types.HijackedResponse{}, fmt.Errorf("conectando no exec %s: %w", execID, err)
	}
	return resp, nil
}

// ExecResize ajusta o tamanho do pseudo-terminal — chamado quando o
// terminal no navegador é redimensionado.
func (c *Client) ExecResize(ctx context.Context, execID string, height, width uint) error {
	if err := c.cli.ContainerExecResize(ctx, execID, types.ResizeOptions{Height: height, Width: width}); err != nil {
		return fmt.Errorf("redimensionando exec %s: %w", execID, err)
	}
	return nil
}

// ExecRun roda um comando síncrono, não-interativo (sem Tty) e devolve
// saída combinada + exit code — usado só pelo delete do file manager (ver
// internal/infra/container_files.go), nunca pelo terminal interativo.
func (c *Client) ExecRun(ctx context.Context, containerID string, cmd []string) (exitCode int, output string, err error) {
	created, err := c.cli.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		return 0, "", fmt.Errorf("criando exec no container %s: %w", containerID, err)
	}

	attached, err := c.cli.ContainerExecAttach(ctx, created.ID, types.ExecStartCheck{})
	if err != nil {
		return 0, "", fmt.Errorf("conectando no exec %s: %w", created.ID, err)
	}
	defer attached.Close()

	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, &buf, attached.Reader); err != nil {
		return 0, "", fmt.Errorf("lendo saída do exec %s: %w", created.ID, err)
	}

	inspect, err := c.cli.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return 0, "", fmt.Errorf("checando resultado do exec %s: %w", created.ID, err)
	}

	return inspect.ExitCode, buf.String(), nil
}
