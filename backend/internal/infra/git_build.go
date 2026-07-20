package infra

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneAndBuild clona um repositório Git (público ou privado, via credencial
// já cadastrada — ver git_credentials.go) num diretório efêmero e builda a
// imagem a partir do Dockerfile na raiz. Reusa o mesmo runBuild de
// build.go — clonar é só uma forma alternativa de montar o contexto de
// build, o resto do caminho é idêntico ao de BuildFromContext.
func (s *Service) CloneAndBuild(ctx context.Context, tag, repoURL, branch string, credentialID *string) (*BuildResult, error) {
	if !imageTagRegex.MatchString(tag) {
		return nil, fmt.Errorf("tag de imagem inválida")
	}
	if strings.TrimSpace(repoURL) == "" {
		return nil, fmt.Errorf("URL do repositório é obrigatória")
	}
	if branch == "" {
		branch = "main"
	}

	cloneURL := repoURL
	var extraEnv []string
	var cleanup func()

	if credentialID != nil && *credentialID != "" {
		kind, username, secret, err := s.resolveGitCredential(ctx, *credentialID)
		if err != nil {
			return nil, err
		}
		switch kind {
		case GitCredentialSSHKey:
			keyFile, err := os.CreateTemp("", "gestpg-git-key-*")
			if err != nil {
				return nil, fmt.Errorf("preparando chave SSH temporária: %w", err)
			}
			if _, err := keyFile.WriteString(secret); err != nil {
				keyFile.Close()
				os.Remove(keyFile.Name())
				return nil, fmt.Errorf("gravando chave SSH temporária: %w", err)
			}
			keyFile.Close()
			if err := os.Chmod(keyFile.Name(), 0o600); err != nil {
				os.Remove(keyFile.Name())
				return nil, fmt.Errorf("ajustando permissão da chave SSH temporária: %w", err)
			}
			keyPath := keyFile.Name()
			cleanup = func() { os.Remove(keyPath) }
			// accept-new + known_hosts em /dev/null: aceita a chave do host na
			// hora sem persistir nada entre chamadas — clone é sempre um
			// processo novo e efêmero, não vale a pena manter known_hosts. É
			// um trade-off consciente (não protege contra MITM entre chamadas
			// diferentes), aceitável pra um clone pontual disparado pelo
			// próprio admin da plataforma.
			extraEnv = append(extraEnv, fmt.Sprintf(
				"GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null -o IdentitiesOnly=yes",
				keyPath,
			))
		case GitCredentialPAT:
			u, err := url.Parse(repoURL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				return nil, fmt.Errorf("credencial de token só funciona com URL https://")
			}
			if username != "" {
				u.User = url.UserPassword(username, secret)
			} else {
				u.User = url.User(secret)
			}
			cloneURL = u.String()
		}
	}

	if cleanup != nil {
		defer cleanup()
	}

	dir := buildDir(tag)
	if err := os.RemoveAll(dir); err != nil {
		return nil, fmt.Errorf("limpando diretório de build anterior: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("criando diretório de build: %w", err)
	}

	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--depth", "1", cloneURL, dir)
	cloneCmd.Env = append(os.Environ(), extraEnv...)
	var cloneLog bytes.Buffer
	cloneCmd.Stdout = &cloneLog
	cloneCmd.Stderr = &cloneLog
	if err := cloneCmd.Run(); err != nil {
		return &BuildResult{Tag: tag, Log: cloneLog.String(), Success: false}, fmt.Errorf("git clone falhou — veja o log")
	}

	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err != nil {
		return &BuildResult{Tag: tag, Log: cloneLog.String(), Success: false}, fmt.Errorf("repositório clonado não tem Dockerfile na raiz")
	}

	return s.runBuild(ctx, tag, dir)
}

type CreateContainerFromGitInput struct {
	Name         string            `json:"name"`
	Tag          string            `json:"tag"`
	RepoURL      string            `json:"repo_url"`
	Branch       string            `json:"branch"`
	CredentialID *string           `json:"credential_id,omitempty"`
	Env          map[string]string `json:"env"`
	Ports        map[string]int    `json:"ports"`
	NetworkName  string            `json:"network"`
}

// CreateContainerFromGit encadeia CloneAndBuild -> CreateContainerFromImage.
// Se o build falhar, devolve o log de qualquer jeito (mesma convenção 422
// já usada por BuildFromDockerfile/BuildFromContext) — quem chama decide se
// mostra como erro de domínio ou de protocolo.
func (s *Service) CreateContainerFromGit(ctx context.Context, in CreateContainerFromGitInput) (string, *BuildResult, error) {
	if strings.TrimSpace(in.Name) == "" {
		return "", nil, fmt.Errorf("nome do container é obrigatório")
	}
	result, err := s.CloneAndBuild(ctx, in.Tag, in.RepoURL, in.Branch, in.CredentialID)
	if err != nil {
		return "", result, err
	}
	id, err := s.CreateContainerFromImage(ctx, CreateContainerFromImageInput{
		Name:        in.Name,
		Image:       in.Tag,
		Env:         in.Env,
		Ports:       in.Ports,
		NetworkName: in.NetworkName,
	})
	if err != nil {
		return "", result, err
	}
	return id, result, nil
}
