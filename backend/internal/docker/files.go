package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"

	"github.com/docker/docker/api/types"
)

// ReadFileFromContainer lê um arquivo de dentro do container via a API de
// archive do Docker (GET /containers/{id}/archive) — não é exec, é o mesmo
// mecanismo que `docker cp` usa. O docker-socket-proxy já permite isso pela
// categoria CONTAINERS, sem precisar abrir a categoria EXEC (superfície de
// ataque bem maior: exec roda comando arbitrário, archive só move bytes de
// um arquivo).
func (c *Client) ReadFileFromContainer(ctx context.Context, containerID, filePath string) ([]byte, error) {
	reader, _, err := c.cli.CopyFromContainer(ctx, containerID, filePath)
	if err != nil {
		return nil, fmt.Errorf("lendo %s do container %s: %w", filePath, containerID, err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	if _, err := tr.Next(); err != nil {
		return nil, fmt.Errorf("lendo tar de %s: %w", filePath, err)
	}
	content, err := io.ReadAll(tr)
	if err != nil {
		return nil, fmt.Errorf("lendo conteúdo de %s: %w", filePath, err)
	}
	return content, nil
}

// WriteFileToContainer escreve/sobrescreve um arquivo dentro do container via
// PUT /containers/{id}/archive — mesma família de API do ReadFileFromContainer.
func (c *Client) WriteFileToContainer(ctx context.Context, containerID, filePath string, content []byte, mode int64) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	header := &tar.Header{
		Name: path.Base(filePath),
		Mode: mode,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("montando tar de %s: %w", filePath, err)
	}
	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("escrevendo conteúdo no tar de %s: %w", filePath, err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("fechando tar de %s: %w", filePath, err)
	}

	err := c.cli.CopyToContainer(ctx, containerID, path.Dir(filePath), &buf, types.CopyToContainerOptions{})
	if err != nil {
		return fmt.Errorf("escrevendo %s no container %s: %w", filePath, containerID, err)
	}
	return nil
}
