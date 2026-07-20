package infra

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type GitDeployment struct {
	ID             string            `json:"id"`
	ContainerName  string            `json:"container_name"`
	ImageTag       string            `json:"image_tag"`
	RepoURL        string            `json:"repo_url"`
	Branch         string            `json:"branch"`
	CredentialID   string            `json:"credential_id,omitempty"`
	Env            map[string]string `json:"env"`
	Ports          map[string]int    `json:"ports"`
	NetworkName    string            `json:"network,omitempty"`
	LastDeployedAt *time.Time        `json:"last_deployed_at,omitempty"`
	LastStatus     string            `json:"last_status,omitempty"`
	LastError      string            `json:"last_error,omitempty"`
	LastBuildLog   string            `json:"last_build_log,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

type CreateGitDeploymentInput struct {
	ContainerName string            `json:"container_name"`
	ImageTag      string            `json:"image_tag"`
	RepoURL       string            `json:"repo_url"`
	Branch        string            `json:"branch"`
	CredentialID  string            `json:"credential_id,omitempty"`
	Env           map[string]string `json:"env"`
	Ports         map[string]int    `json:"ports"`
	NetworkName   string            `json:"network,omitempty"`
}

type CreateGitDeploymentResult struct {
	Deployment     GitDeployment `json:"deployment"`
	WebhookURLPath string        `json:"webhook_url_path"`
	// WebhookSecret só existe nessa resposta — nunca mais devolvido depois
	// (mesmo raciocínio de nunca reexibir segredo já salvo usado no resto
	// do projeto). Se perder, exclui e recria o deployment.
	WebhookSecret string `json:"webhook_secret"`
}

func generateWebhookSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// CreateGitDeployment salva a config de clone+build+create de forma
// persistente (diferente de CreateContainerFromGit, que é um disparo único
// sem guardar nada) e já dispara o primeiro deploy na hora.
func (s *Service) CreateGitDeployment(ctx context.Context, in CreateGitDeploymentInput) (*CreateGitDeploymentResult, error) {
	if in.ContainerName == "" || in.ImageTag == "" || in.RepoURL == "" {
		return nil, fmt.Errorf("nome do container, tag da imagem e URL do repositório são obrigatórios")
	}
	if !imageTagRegex.MatchString(in.ImageTag) {
		return nil, fmt.Errorf("tag de imagem inválida")
	}
	branch := in.Branch
	if branch == "" {
		branch = "main"
	}

	secret, err := generateWebhookSecret()
	if err != nil {
		return nil, fmt.Errorf("gerando segredo do webhook: %w", err)
	}
	sealedSecret, err := s.secretBox.Seal(secret)
	if err != nil {
		return nil, fmt.Errorf("cifrando segredo do webhook: %w", err)
	}

	envJSON, _ := json.Marshal(in.Env)
	portsJSON, _ := json.Marshal(in.Ports)

	var credentialID *string
	if in.CredentialID != "" {
		credentialID = &in.CredentialID
	}

	var id string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO git_deployments (container_name, image_tag, repo_url, branch, credential_id, env_json, ports_json, network_name, webhook_secret_encrypted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id::text
	`, in.ContainerName, in.ImageTag, in.RepoURL, branch, credentialID, string(envJSON), string(portsJSON), in.NetworkName, sealedSecret).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("salvando deployment (nome de container já em uso?): %w", err)
	}

	deployErr := s.RedeployFromGit(ctx, id)

	dep, getErr := s.getGitDeployment(ctx, id)
	if getErr != nil {
		return nil, getErr
	}

	result := &CreateGitDeploymentResult{
		Deployment:     *dep,
		WebhookURLPath: fmt.Sprintf("/api/v1/infra/git-deployments/%s/webhook", id),
		WebhookSecret:  secret,
	}
	if deployErr != nil {
		return result, deployErr
	}
	return result, nil
}

func (s *Service) ListGitDeployments(ctx context.Context) ([]GitDeployment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, container_name, image_tag, repo_url, branch, credential_id, env_json, ports_json, network_name,
		       last_deployed_at, last_status, last_error, last_build_log, created_at
		FROM git_deployments ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listando deployments: %w", err)
	}
	defer rows.Close()

	out := []GitDeployment{}
	for rows.Next() {
		d, err := scanGitDeployment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanGitDeployment(row rowScanner) (*GitDeployment, error) {
	var d GitDeployment
	var credID *string
	var envJSON, portsJSON string
	if err := row.Scan(
		&d.ID, &d.ContainerName, &d.ImageTag, &d.RepoURL, &d.Branch, &credID, &envJSON, &portsJSON, &d.NetworkName,
		&d.LastDeployedAt, &d.LastStatus, &d.LastError, &d.LastBuildLog, &d.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("lendo deployment: %w", err)
	}
	if credID != nil {
		d.CredentialID = *credID
	}
	_ = json.Unmarshal([]byte(envJSON), &d.Env)
	_ = json.Unmarshal([]byte(portsJSON), &d.Ports)
	return &d, nil
}

func (s *Service) getGitDeployment(ctx context.Context, id string) (*GitDeployment, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id::text, container_name, image_tag, repo_url, branch, credential_id, env_json, ports_json, network_name,
		       last_deployed_at, last_status, last_error, last_build_log, created_at
		FROM git_deployments WHERE id = $1
	`, id)
	return scanGitDeployment(row)
}

func (s *Service) DeleteGitDeployment(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM git_deployments WHERE id = $1`, id); err != nil {
		return fmt.Errorf("excluindo deployment: %w", err)
	}
	return nil
}

// GitWebhookSecret decifra o segredo pra o handler HTTP validar a
// assinatura do provedor — fica na camada de handler de propósito (evita
// internal/infra depender de net/http só pra isso).
func (s *Service) GitWebhookSecret(ctx context.Context, id string) (string, error) {
	var sealed string
	if err := s.pool.QueryRow(ctx, `SELECT webhook_secret_encrypted FROM git_deployments WHERE id = $1`, id).Scan(&sealed); err != nil {
		return "", fmt.Errorf("lendo deployment: %w", err)
	}
	return s.secretBox.Open(sealed)
}

// RedeployFromGit clona+builda de novo e substitui o container (para o
// antigo se existir, sobe um novo com a mesma config) — chamado tanto pelo
// primeiro deploy (CreateGitDeployment) quanto pelo webhook a cada push,
// quanto por um botão manual "reimplantar agora".
func (s *Service) RedeployFromGit(ctx context.Context, id string) error {
	dep, err := s.getGitDeployment(ctx, id)
	if err != nil {
		return err
	}

	var credID *string
	if dep.CredentialID != "" {
		credID = &dep.CredentialID
	}

	result, buildErr := s.CloneAndBuild(ctx, dep.ImageTag, dep.RepoURL, dep.Branch, credID)
	buildLog := ""
	if result != nil {
		buildLog = result.Log
	}
	if buildErr != nil {
		s.recordGitDeployment(ctx, id, "failed", buildErr.Error(), buildLog)
		return buildErr
	}

	if existingID, findErr := s.docker.FindContainerIDByName(ctx, dep.ContainerName); findErr == nil && existingID != "" {
		_ = s.docker.RemoveContainer(ctx, existingID, "", false)
	}

	_, createErr := s.CreateContainerFromImage(ctx, CreateContainerFromImageInput{
		Name:        dep.ContainerName,
		Image:       dep.ImageTag,
		Env:         dep.Env,
		Ports:       dep.Ports,
		NetworkName: dep.NetworkName,
	})
	if createErr != nil {
		s.recordGitDeployment(ctx, id, "failed", createErr.Error(), buildLog)
		return createErr
	}

	s.recordGitDeployment(ctx, id, "success", "", buildLog)
	return nil
}

func (s *Service) recordGitDeployment(ctx context.Context, id, status, errMsg, buildLog string) {
	_, _ = s.pool.Exec(ctx, `
		UPDATE git_deployments SET last_deployed_at = now(), last_status = $2, last_error = $3, last_build_log = $4, updated_at = now()
		WHERE id = $1
	`, id, status, errMsg, buildLog)
}
