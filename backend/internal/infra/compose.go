package infra

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
)

const stacksBaseDir = "/stacks"

// composeNameRegex é a regra de nome de projeto do próprio `docker compose`
// (minúsculo, começa com letra/dígito, só letra/dígito/traço/underscore) —
// validado aqui também porque o nome vira caminho de diretório e argumento
// de shell.
var composeNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

type ComposeProject struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	ComposeContent string    `json:"compose_content"`
	Status         string    `json:"status"`
	LastError      string    `json:"last_error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func composeFilePath(name string) string {
	return filepath.Join(stacksBaseDir, name, "docker-compose.yml")
}

// DeployCompose escreve o YAML no volume de stacks e roda `docker compose up
// -d` via os/exec — o próprio compose cuida de rede/depends_on/healthcheck
// direito, não vale a pena reimplementar esse parser. Idempotente: chamar de
// novo com conteúdo novo atualiza o stack (mesmo comportamento de sempre do
// `docker compose up -d`, recria só o que mudou).
func (s *Service) DeployCompose(ctx context.Context, name, content string) (*ComposeProject, error) {
	if !composeNameRegex.MatchString(name) {
		return nil, fmt.Errorf("nome do stack inválido — use minúsculas, dígitos, traço ou underscore")
	}
	if content == "" {
		return nil, fmt.Errorf("conteúdo do docker-compose.yml é obrigatório")
	}

	path := composeFilePath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("criando diretório do stack: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("gravando docker-compose.yml: %w", err)
	}

	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", name, "-f", path, "up", "-d", "--remove-orphans")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	status := "deployed"
	lastError := ""
	if runErr != nil {
		status = "error"
		lastError = stderr.String()
		if lastError == "" {
			lastError = runErr.Error()
		}
	}

	var p ComposeProject
	err := s.pool.QueryRow(ctx, `
		INSERT INTO compose_projects (name, compose_content, status, last_error)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name) DO UPDATE SET
			compose_content = $2, status = $3, last_error = $4, updated_at = now()
		RETURNING id, name, compose_content, status, last_error, created_at, updated_at
	`, name, content, status, lastError).Scan(
		&p.ID, &p.Name, &p.ComposeContent, &p.Status, &p.LastError, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("salvando registro do stack: %w", err)
	}
	if runErr != nil {
		return &p, fmt.Errorf("docker compose up falhou: %s", lastError)
	}
	return &p, nil
}

func (s *Service) ListComposeProjects(ctx context.Context) ([]ComposeProject, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, compose_content, status, last_error, created_at, updated_at
		FROM compose_projects ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listando stacks: %w", err)
	}
	defer rows.Close()

	out := make([]ComposeProject, 0)
	for rows.Next() {
		var p ComposeProject
		if err := rows.Scan(&p.ID, &p.Name, &p.ComposeContent, &p.Status, &p.LastError, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("lendo stack: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) getComposeProject(ctx context.Context, name string) (*ComposeProject, error) {
	var p ComposeProject
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, compose_content, status, last_error, created_at, updated_at
		FROM compose_projects WHERE name = $1
	`, name).Scan(&p.ID, &p.Name, &p.ComposeContent, &p.Status, &p.LastError, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("stack %q não encontrado", name)
		}
		return nil, fmt.Errorf("lendo stack: %w", err)
	}
	return &p, nil
}

// RemoveComposeProject roda `docker compose down` (derruba containers +
// redes próprias do stack) e, se removeVolumes, `-v` junto (apaga volumes
// nomeados que o compose criou pra esse stack — irreversível, decisão
// explícita de quem chama).
func (s *Service) RemoveComposeProject(ctx context.Context, name string, removeVolumes bool) error {
	p, err := s.getComposeProject(ctx, name)
	if err != nil {
		return err
	}
	path := composeFilePath(p.Name)

	args := []string{"compose", "-p", p.Name, "-f", path, "down"}
	if removeVolumes {
		args = append(args, "-v")
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down falhou: %s", stderr.String())
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM compose_projects WHERE name = $1`, p.Name); err != nil {
		return fmt.Errorf("excluindo registro do stack: %w", err)
	}
	os.RemoveAll(filepath.Dir(path))
	return nil
}
