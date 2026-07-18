package infra

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var imageTagRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]*[a-z0-9])?(:[a-zA-Z0-9._-]+)?$`)

type BuildResult struct {
	Tag     string `json:"tag"`
	Log     string `json:"log"`
	Success bool   `json:"success"`
}

func buildDir(tag string) string {
	safe := strings.NewReplacer("/", "_", ":", "_").Replace(tag)
	return filepath.Join(stacksBaseDir, "builds", safe)
}

// BuildFromDockerfile builda sem contexto extra além do próprio Dockerfile —
// cobre o caso comum de um Dockerfile que só faz FROM/RUN/ENV/CMD, sem COPY
// de arquivo do usuário.
func (s *Service) BuildFromDockerfile(ctx context.Context, tag, dockerfileContent string) (*BuildResult, error) {
	if !imageTagRegex.MatchString(tag) {
		return nil, fmt.Errorf("tag de imagem inválida")
	}
	if strings.TrimSpace(dockerfileContent) == "" {
		return nil, fmt.Errorf("Dockerfile é obrigatório")
	}
	dir := buildDir(tag)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("criando diretório de build: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfileContent), 0o644); err != nil {
		return nil, fmt.Errorf("gravando Dockerfile: %w", err)
	}
	return s.runBuild(ctx, tag, dir)
}

// BuildFromContext extrai um tar (opcionalmente .gz) enviado pelo usuário —
// precisa ter um Dockerfile na raiz — e builda com esse contexto completo,
// pro caso que precisa de COPY de arquivo de verdade (não só o Dockerfile
// sozinho, que é o que BuildFromDockerfile cobre).
func (s *Service) BuildFromContext(ctx context.Context, tag string, archive io.Reader, gzipped bool) (*BuildResult, error) {
	if !imageTagRegex.MatchString(tag) {
		return nil, fmt.Errorf("tag de imagem inválida")
	}
	dir := buildDir(tag)
	if err := os.RemoveAll(dir); err != nil {
		return nil, fmt.Errorf("limpando diretório de build anterior: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("criando diretório de build: %w", err)
	}

	var tr *tar.Reader
	if gzipped {
		gz, err := gzip.NewReader(archive)
		if err != nil {
			return nil, fmt.Errorf("descompactando contexto: %w", err)
		}
		defer gz.Close()
		tr = tar.NewReader(gz)
	} else {
		tr = tar.NewReader(archive)
	}

	if err := extractTar(tr, dir); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err != nil {
		return nil, fmt.Errorf("contexto enviado não tem Dockerfile na raiz")
	}

	return s.runBuild(ctx, tag, dir)
}

// extractTar valida cada entrada contra zip-slip (caminho que escapa do
// diretório de destino via ../) antes de escrever qualquer coisa — o tar
// vem de upload do usuário, não é conteúdo confiável por padrão.
func extractTar(tr *tar.Reader, dir string) error {
	cleanDir := filepath.Clean(dir)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("lendo contexto de build: %w", err)
		}
		target := filepath.Join(dir, hdr.Name)
		if target != cleanDir && !strings.HasPrefix(target, cleanDir+string(os.PathSeparator)) {
			return fmt.Errorf("contexto de build contém caminho inválido: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
}

func (s *Service) runBuild(ctx context.Context, tag, dir string) (*BuildResult, error) {
	cmd := exec.CommandContext(ctx, "docker", "build", "-t", tag, dir)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	result := &BuildResult{Tag: tag, Log: out.String(), Success: err == nil}
	if err != nil {
		return result, fmt.Errorf("build falhou — veja o log")
	}
	return result, nil
}
