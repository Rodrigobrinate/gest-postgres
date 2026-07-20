package infra

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// scpLikeGitURLRegex casa a sintaxe curta de SSH que o próprio git aceita
// (ex: "git@github.com:org/repo.git") — sem esquema, então não passa pelo
// url.Parse/allowlist de scheme abaixo. Só usada como alternativa quando o
// valor não tem "://" nenhum.
var scpLikeGitURLRegex = regexp.MustCompile(`^[A-Za-z0-9_.-]+@[A-Za-z0-9_.-]+:[A-Za-z0-9_./-]+$`)
var gitBranchRegex = regexp.MustCompile(`^[A-Za-z0-9._][A-Za-z0-9._/-]*$`)

// allowedGitSchemes é a allowlist de transporte — cada um dos que faltam
// (mais notavelmente "ext" e "file") vira execução de comando ou leitura
// arbitrária de arquivo do próprio container do backend quando passado cru
// pro `git clone`.
var allowedGitSchemes = map[string]bool{"http": true, "https": true, "ssh": true, "git": true}

// validateGitRepoURL fecha o RCE via transporte `ext::`/injeção de argumento
// por traço — precisa rodar ANTES de qualquer outro processamento do
// repoURL (inclusive do caminho PAT, que hoje é o único que faz url.Parse),
// porque o valor cru chega tanto de criação quanto do redeploy via webhook
// (sem sessão, sem esse gate seria a única defesa).
func validateGitRepoURL(repoURL string) error {
	if strings.Contains(repoURL, "::") {
		return fmt.Errorf("URL de repositório inválida")
	}
	if strings.HasPrefix(repoURL, "-") {
		return fmt.Errorf("URL de repositório inválida")
	}
	if strings.Contains(repoURL, "://") {
		u, err := url.Parse(repoURL)
		if err != nil || !allowedGitSchemes[u.Scheme] || u.Host == "" {
			return fmt.Errorf("URL de repositório inválida — esquemas aceitos: http, https, ssh")
		}
		return nil
	}
	if !scpLikeGitURLRegex.MatchString(repoURL) {
		return fmt.Errorf("URL de repositório inválida")
	}
	return nil
}

func validateGitBranch(branch string) error {
	if !gitBranchRegex.MatchString(branch) {
		return fmt.Errorf("branch inválido")
	}
	return nil
}

// credentialInURLRegex casa "user:senha@" ou "user@" numa URL — defesa em
// profundidade pro log de clone (persistido em git_deployments.last_build_log
// e devolvido na resposta da API): mesmo não embutindo mais credencial na
// URL por padrão (ver CloneAndBuild), se o git ecoar alguma URL com
// userinfo em algum erro, não vaza no log guardado.
var credentialInURLRegex = regexp.MustCompile(`://[^/@\s]+@`)

func redactCredentialsInLog(log string) string {
	return credentialInURLRegex.ReplaceAllString(log, "://<redacted>@")
}

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
	if err := validateGitRepoURL(repoURL); err != nil {
		return nil, err
	}
	if branch == "" {
		branch = "main"
	}
	if err := validateGitBranch(branch); err != nil {
		return nil, err
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
			// Nunca embute o token na URL — a URL vira argv do processo
			// `git clone` (legível por qualquer processo no container via
			// /proc/<pid>/cmdline) e também fica presa em `cloneLog` se o
			// git ecoar a URL num erro. Só o USERNAME (ou um placeholder
			// convencional aceito por GitHub/GitLab/Bitbucket quando o token
			// já basta sozinho) vai na URL; o segredo passa por
			// GIT_ASKPASS, mesma classe de exposição (env var) já aceita
			// pro path da chave SSH acima — nunca argv.
			effectiveUsername := username
			if effectiveUsername == "" {
				effectiveUsername = "x-access-token"
			}
			u.User = url.User(effectiveUsername)
			cloneURL = u.String()

			askpassFile, err := os.CreateTemp("", "gestpg-git-askpass-*.sh")
			if err != nil {
				return nil, fmt.Errorf("preparando askpass temporário: %w", err)
			}
			if _, err := askpassFile.WriteString("#!/bin/sh\nprintf '%s' \"$GESTPG_GIT_PASSWORD\"\n"); err != nil {
				askpassFile.Close()
				os.Remove(askpassFile.Name())
				return nil, fmt.Errorf("gravando askpass temporário: %w", err)
			}
			askpassFile.Close()
			if err := os.Chmod(askpassFile.Name(), 0o700); err != nil {
				os.Remove(askpassFile.Name())
				return nil, fmt.Errorf("ajustando permissão do askpass temporário: %w", err)
			}
			askpassPath := askpassFile.Name()
			cleanup = func() { os.Remove(askpassPath) }
			extraEnv = append(extraEnv,
				"GIT_ASKPASS="+askpassPath,
				"GESTPG_GIT_PASSWORD="+secret,
			)
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

	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--depth", "1", "--", cloneURL, dir)
	// GIT_ALLOW_PROTOCOL é defesa em profundidade: mesmo que algum bypass
	// escape da validação acima, o próprio git recusa qualquer transporte
	// fora dessa lista (mata ext:: e file:: na raiz).
	cloneCmd.Env = append(append(os.Environ(), extraEnv...), "GIT_ALLOW_PROTOCOL=http:https:ssh:git")
	var cloneLog bytes.Buffer
	cloneCmd.Stdout = &cloneLog
	cloneCmd.Stderr = &cloneLog
	if err := cloneCmd.Run(); err != nil {
		return &BuildResult{Tag: tag, Log: redactCredentialsInLog(cloneLog.String()), Success: false}, fmt.Errorf("git clone falhou — veja o log")
	}

	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err != nil {
		return &BuildResult{Tag: tag, Log: redactCredentialsInLog(cloneLog.String()), Success: false}, fmt.Errorf("repositório clonado não tem Dockerfile na raiz")
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
	CPUCores     float64           `json:"cpu_cores,omitempty"`
	MemoryMB     int               `json:"memory_mb,omitempty"`
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
		CPUCores:    in.CPUCores,
		MemoryMB:    in.MemoryMB,
	})
	if err != nil {
		return "", result, err
	}
	return id, result, nil
}
