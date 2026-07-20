package infra

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// hostFilesRoot é o ponto de montagem FIXO dentro do container do backend —
// o bind mount de verdade (HOST_FILES_ROOT do .env, default
// /srv/gestpg-files no HOST) fica em docker-compose.yml. Nunca é a raiz "/"
// do host: essa é uma decisão consciente (não pedida explicitamente, mas
// necessária) — um bind mount read-write da raiz inteira seria root via
// browser. A raiz configurável já É o limite; não tem "fora dela" alcançável
// de dentro do container pra checar.
const hostFilesRoot = "/hostfiles"

type HostFileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	Mode    int64     `json:"mode"`
	ModTime time.Time `json:"mod_time"`
}

// resolveHostPath junta o caminho pedido com a raiz fixa e garante que o
// resultado nunca escapa dela — nem por "../" literal (checagem de prefixo,
// mesmo idioma do zip-slip guard de build.go) nem por symlink apontando pra
// fora (EvalSymlinks + re-checagem — mais forte que o guard de build.go
// porque esse aqui é um navegador ao vivo, não uma extração de tar única).
func resolveHostPath(userPath string) (string, error) {
	clean := filepath.Clean("/" + userPath)
	joined := filepath.Join(hostFilesRoot, clean)
	if joined != hostFilesRoot && !strings.HasPrefix(joined, hostFilesRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("caminho inválido")
	}

	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("resolvendo caminho: %w", err)
		}
		// Caminho ainda não existe (vai ser criado por write/upload) — valida
		// o diretório pai, que já deve existir.
		parent, err2 := filepath.EvalSymlinks(filepath.Dir(joined))
		if err2 != nil {
			return "", fmt.Errorf("caminho inválido: %w", err2)
		}
		if parent != hostFilesRoot && !strings.HasPrefix(parent, hostFilesRoot+string(os.PathSeparator)) {
			return "", fmt.Errorf("caminho inválido")
		}
		return joined, nil
	}
	if resolved != hostFilesRoot && !strings.HasPrefix(resolved, hostFilesRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("caminho inválido")
	}
	return joined, nil
}

func (s *Service) ListHostDirectory(ctx context.Context, userPath string) ([]HostFileEntry, error) {
	full, err := resolveHostPath(userPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, fmt.Errorf("listando %s: %w", userPath, err)
	}
	out := make([]HostFileEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, HostFileEntry{
			Name:    e.Name(),
			Path:    path.Join("/", userPath, e.Name()),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			Mode:    int64(info.Mode().Perm()),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *Service) StatHostPath(ctx context.Context, userPath string) (HostFileEntry, error) {
	full, err := resolveHostPath(userPath)
	if err != nil {
		return HostFileEntry{}, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return HostFileEntry{}, fmt.Errorf("consultando %s: %w", userPath, err)
	}
	return HostFileEntry{
		Name:    info.Name(),
		Path:    userPath,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		Mode:    int64(info.Mode().Perm()),
		ModTime: info.ModTime(),
	}, nil
}

func (s *Service) ReadHostFile(ctx context.Context, userPath string) ([]byte, error) {
	full, err := resolveHostPath(userPath)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("lendo %s: %w", userPath, err)
	}
	return content, nil
}

func (s *Service) WriteHostFile(ctx context.Context, userPath string, content []byte) error {
	full, err := resolveHostPath(userPath)
	if err != nil {
		return err
	}
	if full == hostFilesRoot {
		return fmt.Errorf("não é permitido operar na raiz")
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("preparando diretório: %w", err)
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		return fmt.Errorf("escrevendo %s: %w", userPath, err)
	}
	return nil
}

func (s *Service) UploadHostFile(ctx context.Context, destDirPath, filename string, content io.Reader) error {
	if err := validateFilename(filename); err != nil {
		return err
	}
	fullDir, err := resolveHostPath(destDirPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return fmt.Errorf("preparando diretório: %w", err)
	}
	full := filepath.Join(fullDir, filename)
	if !strings.HasPrefix(full, hostFilesRoot+string(os.PathSeparator)) {
		return fmt.Errorf("caminho inválido")
	}
	f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("criando %s: %w", filename, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, content); err != nil {
		return fmt.Errorf("gravando %s: %w", filename, err)
	}
	return nil
}

func (s *Service) DeleteHostPath(ctx context.Context, userPath string) error {
	full, err := resolveHostPath(userPath)
	if err != nil {
		return err
	}
	if full == hostFilesRoot {
		return fmt.Errorf("não é permitido operar na raiz")
	}
	if err := os.RemoveAll(full); err != nil {
		return fmt.Errorf("removendo %s: %w", userPath, err)
	}
	return nil
}

// ResolvedHostPath expõe o caminho real (já validado) só pra streaming de
// download (zip de pasta ou cópia crua de arquivo) — quem chama é sempre o
// handler HTTP, nunca atravessa camada nenhuma além disso.
func (s *Service) ResolvedHostPath(ctx context.Context, userPath string) (string, error) {
	return resolveHostPath(userPath)
}
