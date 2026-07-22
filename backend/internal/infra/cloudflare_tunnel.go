package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/gest-postgres/backend/internal/docker"
	"github.com/jackc/pgx/v5"
)

const cloudflareTunnelContainerName = "gestpg-cloudflared"

// cloudflareTunnelImage é tag fixa, não ":latest" — mesma disciplina de
// supply-chain do traefik:v3.2 (image pinada em traefik.go). Bump manual
// periódico, não automático.
const cloudflareTunnelImage = "cloudflare/cloudflared:2025.6.1"

type CloudflareTunnelStatus struct {
	Enabled bool `json:"enabled"`
	Running bool `json:"running"`
}

func (s *Service) CloudflareTunnelStatus(ctx context.Context) (*CloudflareTunnelStatus, error) {
	var enabled bool
	var containerID string
	err := s.pool.QueryRow(ctx, `SELECT cloudflare_tunnel_enabled, cloudflare_tunnel_container_id FROM platform_settings WHERE id = 1`).
		Scan(&enabled, &containerID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("lendo config do túnel cloudflare: %w", err)
	}
	status := &CloudflareTunnelStatus{Enabled: enabled}
	if enabled && containerID != "" {
		if info, err := s.docker.InspectContainer(ctx, containerID); err == nil {
			status.Running = info.Running
		}
	}
	return status, nil
}

// EnableCloudflareTunnel sobe o cloudflared apontando pro token de túnel
// gerado no painel da Cloudflare (Zero Trust > Tunnels) — outbound-only, sem
// porta nenhuma publicada: alcança backend:28080 só por nome de container na
// rede gestpg-internal. Diferente do Traefik (que abre 80/443 pro mundo), a
// superfície aqui é zero portas — é literalmente o ponto do túnel.
func (s *Service) EnableCloudflareTunnel(ctx context.Context, token string) (*CloudflareTunnelStatus, error) {
	if token == "" {
		return nil, fmt.Errorf("token do túnel não pode ser vazio")
	}

	if err := s.docker.PullImageIfMissing(ctx, cloudflareTunnelImage); err != nil {
		return nil, fmt.Errorf("baixando imagem do cloudflared: %w", err)
	}

	containerID, err := s.docker.CreateGenericContainer(ctx, docker.CreateGenericContainerInput{
		Name:  cloudflareTunnelContainerName,
		Image: cloudflareTunnelImage,
		// Token via env, não --token na linha de comando — não aparece em
		// `docker ps`/inspect de comando, mesma disciplina de nunca deixar
		// segredo visível em argv usada no resto do projeto (ex: PAT do git,
		// ver git_credentials.go).
		Command:              []string{"tunnel", "--no-autoupdate", "run"},
		Env:                  []string{"TUNNEL_TOKEN=" + token},
		NetworkName:          s.networkName,
		Labels:               map[string]string{docker.LabelManaged: "true"},
		RestartUnlessStopped: true,
	})
	if err != nil {
		return nil, fmt.Errorf("criando container do cloudflared: %w", err)
	}

	sealed, err := s.secretBox.Seal(token)
	if err != nil {
		return nil, fmt.Errorf("cifrando token do túnel: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO platform_settings (id, cloudflare_tunnel_enabled, cloudflare_tunnel_token_encrypted, cloudflare_tunnel_container_id)
		VALUES (1, true, $1, $2)
		ON CONFLICT (id) DO UPDATE SET
			cloudflare_tunnel_enabled = true,
			cloudflare_tunnel_token_encrypted = $1,
			cloudflare_tunnel_container_id = $2,
			updated_at = now()
	`, sealed, containerID)
	if err != nil {
		return nil, fmt.Errorf("salvando config do túnel cloudflare: %w", err)
	}

	// Não WaitHealthy: cloudflared não expõe healthcheck HTTP como o
	// Traefik — conectividade de verdade só se confirma no painel Zero
	// Trust do lado da Cloudflare, fora do alcance dessa chamada.
	time.Sleep(2 * time.Second)
	return s.CloudflareTunnelStatus(ctx)
}

// DisableCloudflareTunnel remove o container — nada mais fica retido (ao
// contrário do Traefik, não existe volume/certificado pra preservar aqui,
// o túnel é inteiramente stateless do lado do servidor).
func (s *Service) DisableCloudflareTunnel(ctx context.Context) error {
	var containerID string
	err := s.pool.QueryRow(ctx, `SELECT cloudflare_tunnel_container_id FROM platform_settings WHERE id = 1`).Scan(&containerID)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("lendo config do túnel cloudflare: %w", err)
	}
	if containerID != "" {
		if err := s.docker.RemoveContainer(ctx, containerID, "", false); err != nil {
			return err
		}
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE platform_settings SET cloudflare_tunnel_enabled = false, cloudflare_tunnel_container_id = '', updated_at = now() WHERE id = 1
	`)
	if err != nil {
		return fmt.Errorf("salvando config do túnel cloudflare: %w", err)
	}
	return nil
}
