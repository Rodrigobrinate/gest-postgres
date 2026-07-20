package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

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

// FileEntry é um filho direto (não recursivo) de um diretório listado — ver
// ListDirectoryInContainer.
type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	Mode    int64     `json:"mode"`
	ModTime time.Time `json:"mod_time"`
}

// StatPathInContainer consulta metadado de um caminho sem transferir
// conteúdo (HEAD, não GET) — usado pra decidir arquivo-vs-pasta antes de
// baixar, e pra tela de "propriedades".
func (c *Client) StatPathInContainer(ctx context.Context, containerID, targetPath string) (types.ContainerPathStat, error) {
	stat, err := c.cli.ContainerStatPath(ctx, containerID, targetPath)
	if err != nil {
		return types.ContainerPathStat{}, fmt.Errorf("consultando %s no container %s: %w", targetPath, containerID, err)
	}
	return stat, nil
}

// ListDirectoryInContainer lista os filhos diretos de um diretório — a API
// de archive do Docker não tem uma operação de listagem própria, então isso
// pede o tar da pasta inteira e lê só os HEADERS (nunca o corpo dos
// arquivos, tar.Reader.Next() pula o resto sem precisar ler), filtrando pra
// manter só um nível — não a árvore recursiva inteira.
func (c *Client) ListDirectoryInContainer(ctx context.Context, containerID, dirPath string) ([]FileEntry, error) {
	reader, _, err := c.cli.CopyFromContainer(ctx, containerID, dirPath)
	if err != nil {
		return nil, fmt.Errorf("listando %s no container %s: %w", dirPath, containerID, err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	entries := []FileEntry{}
	seen := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("lendo listagem de %s: %w", dirPath, err)
		}
		// O primeiro componente do nome no tar é sempre a pasta pedida —
		// remove ele e mantém só quem não tem mais nenhuma "/" depois
		// (filho direto, não neto).
		name := strings.TrimSuffix(hdr.Name, "/")
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 || parts[1] == "" || strings.Contains(parts[1], "/") {
			continue
		}
		child := parts[1]
		if seen[child] {
			continue
		}
		seen[child] = true
		entries = append(entries, FileEntry{
			Name:    child,
			Path:    path.Join(dirPath, child),
			IsDir:   hdr.Typeflag == tar.TypeDir,
			Size:    hdr.Size,
			Mode:    hdr.Mode,
			ModTime: hdr.ModTime,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// DownloadFromContainer devolve o stream tar cru + o stat do caminho pedido.
// Quem chama decide (via stat.Mode.IsDir()) se desembrulha um arquivo único
// ou serve o tar como download de pasta.
func (c *Client) DownloadFromContainer(ctx context.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
	reader, stat, err := c.cli.CopyFromContainer(ctx, containerID, srcPath)
	if err != nil {
		return nil, types.ContainerPathStat{}, fmt.Errorf("baixando %s do container %s: %w", srcPath, containerID, err)
	}
	return reader, stat, nil
}

// UploadFileToContainer envia um arquivo pro container a partir de um
// io.Reader (nunca bufferiza o arquivo inteiro em memória — monta o tar em
// streaming via io.Pipe, mesmo raciocínio do upload de backup pro Google
// Drive em internal/server/backup.go).
func (c *Client) UploadFileToContainer(ctx context.Context, containerID, destDir, filename string, content io.Reader, size, mode int64) error {
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)

	go func() {
		err := func() error {
			if err := tw.WriteHeader(&tar.Header{Name: filename, Mode: mode, Size: size}); err != nil {
				return err
			}
			if _, err := io.Copy(tw, content); err != nil {
				return err
			}
			return tw.Close()
		}()
		pw.CloseWithError(err)
	}()

	if err := c.cli.CopyToContainer(ctx, containerID, destDir, pr, types.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("enviando %s pro container %s: %w", filename, containerID, err)
	}
	return nil
}

// UploadArchiveToContainer extrai um tar cru (já descompactado por quem
// chama, se preciso) dentro de destDir no container — é o reverso de
// DownloadFromContainer, usado pra restaurar snapshot de volume (ver
// internal/infra/volume_backups.go RestoreVolumeBackup).
func (c *Client) UploadArchiveToContainer(ctx context.Context, containerID, destDir string, tarStream io.Reader) error {
	if err := c.cli.CopyToContainer(ctx, containerID, destDir, tarStream, types.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("restaurando arquivos no container %s: %w", containerID, err)
	}
	return nil
}

// ClearDirectoryInContainer apaga só o CONTEÚDO de um diretório (não o
// diretório em si) — via exec síncrono, mesmo raciocínio de
// DeleteInContainer. Usado antes de restaurar backup por cima de um volume
// já existente, pra não deixar arquivo velho que não tava no snapshot.
func (c *Client) ClearDirectoryInContainer(ctx context.Context, containerID, dirPath string) error {
	// find -mindepth 1 -delete apaga tudo dentro (incluindo dotfile), sem
	// depender de expansão de glob do shell (que se comporta diferente
	// entre sh/bash pra dotfile).
	exitCode, output, err := c.ExecRun(ctx, containerID, []string{"find", dirPath, "-mindepth", "1", "-delete"})
	if err != nil {
		return fmt.Errorf("limpando %s no container %s: %w", dirPath, containerID, err)
	}
	if exitCode != 0 {
		return fmt.Errorf("limpando %s no container %s: %s", dirPath, containerID, strings.TrimSpace(output))
	}
	return nil
}

// DeleteInContainer roda um `rm -rf` via exec síncrono — a API de archive
// não tem operação de exclusão. Path já deve vir validado por quem chama
// (nunca vazio, nunca "/"). Depende da categoria EXEC do docker-socket-proxy
// (ver docker-compose.yml) — única parte do file manager que não fica só no
// archive API, de propósito documentado (ver internal/infra/container_files.go).
func (c *Client) DeleteInContainer(ctx context.Context, containerID, targetPath string) error {
	exitCode, output, err := c.ExecRun(ctx, containerID, []string{"rm", "-rf", "--", targetPath})
	if err != nil {
		return fmt.Errorf("removendo %s no container %s: %w", targetPath, containerID, err)
	}
	if exitCode != 0 {
		return fmt.Errorf("removendo %s no container %s: %s", targetPath, containerID, strings.TrimSpace(output))
	}
	return nil
}
