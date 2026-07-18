package infra

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gest-postgres/backend/internal/docker"
	"github.com/jackc/pgx/v5"
)

const traefikContainerName = "gestpg-traefik"

type TraefikStatus struct {
	Enabled   bool   `json:"enabled"`
	Running   bool   `json:"running"`
	AcmeEmail string `json:"acme_email,omitempty"`
}

func (s *Service) TraefikStatus(ctx context.Context) (*TraefikStatus, error) {
	var enabled bool
	var containerID, email string
	err := s.pool.QueryRow(ctx, `SELECT traefik_enabled, traefik_container_id, acme_email FROM platform_settings WHERE id = 1`).
		Scan(&enabled, &containerID, &email)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("lendo config do traefik: %w", err)
	}
	status := &TraefikStatus{Enabled: enabled, AcmeEmail: email}
	if enabled && containerID != "" {
		if info, err := s.docker.InspectContainer(ctx, containerID); err == nil {
			status.Running = info.Running
		}
	}
	return status, nil
}

var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// EnableTraefik sobe o container do Traefik — só provider file (rotas
// dinâmicas escritas por CreateProxyRoute, sem precisar recriar o container
// alvo). De propósito SEM o provider docker: ele exigiria dar ao Traefik
// acesso de leitura ao Docker (direto ou via docker-socket-proxy numa rede
// que hoje só backend/metadata-db alcançam), e o file provider já cobre tudo
// que essa tela de proxy precisa — não vale a superfície extra por uma
// capacidade redundante. ACME via HTTP-01: não precisa de credencial de
// provedor de DNS, só a porta 80 alcançável da internet — pré-requisito que
// qualquer droplet público já atende.
func (s *Service) EnableTraefik(ctx context.Context, acmeEmail string) (*TraefikStatus, error) {
	if !emailRegex.MatchString(acmeEmail) {
		return nil, fmt.Errorf("e-mail inválido — necessário pro registro no Let's Encrypt")
	}

	image := "traefik:v3.2"
	if err := s.docker.PullImageIfMissing(ctx, image); err != nil {
		return nil, fmt.Errorf("baixando imagem do traefik: %w", err)
	}

	containerID, err := s.docker.CreateGenericContainer(ctx, docker.CreateGenericContainerInput{
		Name:  traefikContainerName,
		Image: image,
		Command: []string{
			"--providers.file.directory=/dynamic",
			"--providers.file.watch=true",
			"--entrypoints.web.address=:80",
			"--entrypoints.websecure.address=:443",
			"--certificatesresolvers.le.acme.email=" + acmeEmail,
			"--certificatesresolvers.le.acme.storage=/letsencrypt/acme.json",
			"--certificatesresolvers.le.acme.httpchallenge=true",
			"--certificatesresolvers.le.acme.httpchallenge.entrypoint=web",
			"--log.level=INFO",
		},
		Ports: map[string]string{
			"80/tcp":  "80",
			"443/tcp": "443",
		},
		Binds: []string{
			"gestpg-traefik-dynamic:/dynamic",
			"gestpg-traefik-letsencrypt:/letsencrypt",
		},
		NetworkName:          s.networkName,
		Labels:               map[string]string{docker.LabelManaged: "true"},
		RestartUnlessStopped: true,
	})
	if err != nil {
		return nil, fmt.Errorf("criando container do traefik: %w", err)
	}

	if err := s.docker.WaitHealthy(ctx, containerID, 30*time.Second); err != nil {
		return nil, fmt.Errorf("traefik não subiu a tempo: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO platform_settings (id, traefik_enabled, traefik_container_id, acme_email)
		VALUES (1, true, $1, $2)
		ON CONFLICT (id) DO UPDATE SET traefik_enabled = true, traefik_container_id = $1, acme_email = $2, updated_at = now()
	`, containerID, acmeEmail)
	if err != nil {
		return nil, fmt.Errorf("salvando config do traefik: %w", err)
	}

	return s.TraefikStatus(ctx)
}

// DisableTraefik remove o container mas NUNCA os volumes (certificados do
// Let's Encrypt e rotas dinâmicas ficam guardados — reabilitar depois não
// perde nada, nem reemite certificado à toa contra o rate limit da
// autoridade certificadora).
func (s *Service) DisableTraefik(ctx context.Context) error {
	var containerID string
	err := s.pool.QueryRow(ctx, `SELECT traefik_container_id FROM platform_settings WHERE id = 1`).Scan(&containerID)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("lendo config do traefik: %w", err)
	}
	if containerID != "" {
		if err := s.docker.RemoveContainer(ctx, containerID, "", false); err != nil {
			return err
		}
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE platform_settings SET traefik_enabled = false, traefik_container_id = '', updated_at = now() WHERE id = 1
	`)
	if err != nil {
		return fmt.Errorf("salvando config do traefik: %w", err)
	}
	return nil
}

type ProxyRoute struct {
	ID              string    `json:"id"`
	Domain          string    `json:"domain"`
	TargetContainer string    `json:"target_container"`
	TargetPort      int       `json:"target_port"`
	TLS             bool      `json:"tls"`
	CreatedAt       time.Time `json:"created_at"`
}

type CreateProxyRouteInput struct {
	Domain          string `json:"domain"`
	TargetContainer string `json:"target_container"`
	TargetPort      int    `json:"target_port"`
	TLS             bool   `json:"tls"`
}

var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$`)
var containerNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

func (s *Service) ListProxyRoutes(ctx context.Context) ([]ProxyRoute, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, domain, target_container, target_port, tls, created_at FROM proxy_routes ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listando rotas: %w", err)
	}
	defer rows.Close()
	out := make([]ProxyRoute, 0)
	for rows.Next() {
		var r ProxyRoute
		if err := rows.Scan(&r.ID, &r.Domain, &r.TargetContainer, &r.TargetPort, &r.TLS, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("lendo rota: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateProxyRoute conecta o container alvo na rede gestpg-managed se ainda
// não estiver nela (reaproveitando o mesmo helper da auto-descoberta — sem
// isso o Traefik, que só está nessa rede, não consegue alcançar o
// container pelo nome) e escreve o arquivo de config dinâmica que o file
// provider do Traefik está observando — nunca precisa recriar o container
// alvo só pra rotear um domínio novo.
func (s *Service) CreateProxyRoute(ctx context.Context, in CreateProxyRouteInput) (*ProxyRoute, error) {
	if !domainRegex.MatchString(in.Domain) {
		return nil, fmt.Errorf("domínio inválido")
	}
	if !containerNameRegex.MatchString(in.TargetContainer) {
		return nil, fmt.Errorf("nome de container inválido")
	}
	if in.TargetPort <= 0 || in.TargetPort > 65535 {
		return nil, fmt.Errorf("porta inválida")
	}

	if err := s.docker.ConnectNetwork(ctx, s.networkName, in.TargetContainer); err != nil {
		return nil, fmt.Errorf("conectando %q à rede gerenciada: %w", in.TargetContainer, err)
	}

	var r ProxyRoute
	err := s.pool.QueryRow(ctx, `
		INSERT INTO proxy_routes (domain, target_container, target_port, tls)
		VALUES ($1, $2, $3, $4)
		RETURNING id, domain, target_container, target_port, tls, created_at
	`, in.Domain, in.TargetContainer, in.TargetPort, in.TLS).Scan(
		&r.ID, &r.Domain, &r.TargetContainer, &r.TargetPort, &r.TLS, &r.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("salvando rota: %w", err)
	}

	if err := writeDynamicConfig(r); err != nil {
		_, _ = s.pool.Exec(ctx, `DELETE FROM proxy_routes WHERE id = $1`, r.ID)
		return nil, err
	}
	return &r, nil
}

func (s *Service) DeleteProxyRoute(ctx context.Context, id string) error {
	var domain string
	err := s.pool.QueryRow(ctx, `SELECT domain FROM proxy_routes WHERE id = $1`, id).Scan(&domain)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("rota não encontrada")
		}
		return fmt.Errorf("lendo rota: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM proxy_routes WHERE id = $1`, id); err != nil {
		return fmt.Errorf("excluindo rota: %w", err)
	}
	return removeDynamicConfig(domain)
}

// routeSafeName vira o nome do router/service dentro do Traefik e do
// arquivo em disco — precisa ser um identificador limpo, o domínio como
// veio (com pontos) não serve como nome de arquivo em todo filesystem.
func routeSafeName(domain string) string {
	return strings.ReplaceAll(domain, ".", "-")
}

func dynamicConfigPath(domain string) string {
	return "/traefik-dynamic/" + routeSafeName(domain) + ".yml"
}

func writeDynamicConfig(r ProxyRoute) error {
	name := routeSafeName(r.Domain)
	entrypoint := "web"
	tlsBlock := ""
	if r.TLS {
		entrypoint = "websecure"
		tlsBlock = "\n      tls:\n        certResolver: le"
	}
	content := fmt.Sprintf(`http:
  routers:
    %s:
      rule: "Host(`+"`%s`"+`)"
      entryPoints:
        - %s
      service: %s%s
  services:
    %s:
      loadBalancer:
        servers:
          - url: "http://%s:%d"
`, name, r.Domain, entrypoint, name, tlsBlock, name, r.TargetContainer, r.TargetPort)

	return os.WriteFile(dynamicConfigPath(r.Domain), []byte(content), 0o644)
}

func removeDynamicConfig(domain string) error {
	err := os.Remove(dynamicConfigPath(domain))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removendo config dinâmica: %w", err)
	}
	return nil
}
