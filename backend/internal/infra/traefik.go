package infra

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gest-postgres/backend/internal/docker"
	"github.com/jackc/pgx/v5"
)

// isInternalHost resolve o host e recusa qualquer IP loopback/link-local/
// privado — sem isso, "destino externo" do Traefik consegue publicar um
// serviço interno (docker-socket-proxy, metadata-db, o próprio backend) sob
// um domínio público. Mesmo raciocínio de validateWebhookURL em
// internal/server/notification_channels.go, duplicado aqui (pacotes
// irmãos, sem um lugar compartilhado óbvio pra isso ainda).
func isInternalHost(host string) bool {
	ips, err := net.LookupIP(host)
	if err != nil {
		return false // não resolveu: deixa o Traefik/DNS lidar na hora do request
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsPrivate() {
			return true
		}
	}
	return false
}

const traefikContainerName = "gestpg-traefik"

type TraefikStatus struct {
	Enabled   bool   `json:"enabled"`
	Running   bool   `json:"running"`
	AcmeEmail string `json:"acme_email,omitempty"`
	// ExternalDetected: já existe um Traefik no host que essa plataforma não
	// criou (ex: o do EasyPanel) — quando true, a UI deve orientar pro modo
	// de rota "via labels" (H mais abaixo) em vez de "Habilitar Traefik", pra
	// nunca subir um segundo Traefik brigando pelas portas 80/443.
	ExternalDetected      bool   `json:"external_detected"`
	ExternalContainerName string `json:"external_container_name,omitempty"`
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
	if name, _, found, err := s.docker.DetectExternalTraefik(ctx); err == nil && found {
		status.ExternalDetected = true
		status.ExternalContainerName = name
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
	ID              string `json:"id"`
	Domain          string `json:"domain"`
	TargetContainer string `json:"target_container,omitempty"`
	TargetPort      int    `json:"target_port,omitempty"`
	// TargetURL é o modo "apontar pra fora" (IP/host externo, qualquer
	// serviço que não seja um container gerenciado por essa rede Docker) —
	// mutuamente exclusivo com TargetContainer/TargetPort e RedirectTarget.
	TargetURL         string `json:"target_url,omitempty"`
	TLS               bool   `json:"tls"`
	PathPrefix        string `json:"path_prefix"`
	StripPrefix       bool   `json:"strip_prefix"`
	RedirectTarget    string `json:"redirect_target,omitempty"`
	RedirectPermanent bool   `json:"redirect_permanent"`
	HTTPSRedirect     bool   `json:"https_redirect"`
	// ViaLabels: rota aplicada como label Docker no container ALVO (recriado
	// pra isso) em vez de arquivo no file provider do gestpg-traefik — modo
	// usado quando existe um Traefik externo (EasyPanel etc) que a
	// plataforma nunca deve recriar/tocar. CertResolver é o nome do resolver
	// ACME já configurado NAQUELE Traefik externo (a plataforma não tem como
	// descobrir isso sozinha); vazio = sem TLS gerenciado por label, o
	// certificado fica por conta do que já existir no Traefik externo.
	ViaLabels    bool      `json:"via_labels"`
	CertResolver string    `json:"cert_resolver,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func (r ProxyRoute) isRedirect() bool { return r.RedirectTarget != "" }
func (r ProxyRoute) isExternal() bool { return r.TargetURL != "" }
func (r ProxyRoute) upstreamURL() string {
	if r.isExternal() {
		return r.TargetURL
	}
	return fmt.Sprintf("http://%s:%d", r.TargetContainer, r.TargetPort)
}

type CreateProxyRouteInput struct {
	Domain          string `json:"domain"`
	TargetContainer string `json:"target_container,omitempty"`
	TargetPort      int    `json:"target_port,omitempty"`
	// TargetURL: aponta o domínio pra qualquer host:porta fora do Docker
	// gerenciado (ex: "http://203.0.113.10:9000") — não conecta rede
	// nenhuma, não valida que seja container, é repassado cru pro Traefik
	// como upstream.
	TargetURL string `json:"target_url,omitempty"`
	TLS       bool   `json:"tls"`
	// PathPrefix vazio vira "/" (raiz) — casa com qualquer path do domínio.
	PathPrefix        string `json:"path_prefix,omitempty"`
	StripPrefix       bool   `json:"strip_prefix,omitempty"`
	RedirectTarget    string `json:"redirect_target,omitempty"`
	RedirectPermanent bool   `json:"redirect_permanent,omitempty"`
	// HTTPSRedirect só faz sentido com TLS=true — adiciona um segundo
	// router no entrypoint "web" que redireciona pra https em vez de
	// deixar a porta 80 sem router nenhum pra esse domínio (comportamento
	// de hoje sem essa opção: TLS=true deixa http:// 404ando).
	HTTPSRedirect bool `json:"https_redirect,omitempty"`
	// ViaLabels: usa o modo de rota por label Docker em vez do file
	// provider do gestpg-traefik — só vale pro modo proxy-pra-container
	// (nunca redirect nem destino externo), pensado pra rotear através de
	// um Traefik externo já existente no host sem nunca recriá-lo.
	ViaLabels    bool   `json:"via_labels,omitempty"`
	CertResolver string `json:"cert_resolver,omitempty"`
}

var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$`)
var containerNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

func (s *Service) ListProxyRoutes(ctx context.Context) ([]ProxyRoute, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, domain, target_container, target_port, target_url, tls,
		       path_prefix, strip_prefix, redirect_target, redirect_permanent, https_redirect,
		       via_labels, cert_resolver, created_at
		FROM proxy_routes ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listando rotas: %w", err)
	}
	defer rows.Close()
	out := make([]ProxyRoute, 0)
	for rows.Next() {
		var r ProxyRoute
		if err := rows.Scan(
			&r.ID, &r.Domain, &r.TargetContainer, &r.TargetPort, &r.TargetURL, &r.TLS,
			&r.PathPrefix, &r.StripPrefix, &r.RedirectTarget, &r.RedirectPermanent, &r.HTTPSRedirect,
			&r.ViaLabels, &r.CertResolver, &r.CreatedAt,
		); err != nil {
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
var pathPrefixRegex = regexp.MustCompile(`^/[a-zA-Z0-9._~/-]*$`)

func (s *Service) CreateProxyRoute(ctx context.Context, in CreateProxyRouteInput) (*ProxyRoute, error) {
	if !domainRegex.MatchString(in.Domain) {
		return nil, fmt.Errorf("domínio inválido")
	}
	pathPrefix := in.PathPrefix
	if pathPrefix == "" {
		pathPrefix = "/"
	}
	if !pathPrefixRegex.MatchString(pathPrefix) {
		return nil, fmt.Errorf("caminho inválido — precisa começar com /")
	}

	isRedirect := in.RedirectTarget != ""
	isExternal := in.TargetURL != ""
	if isRedirect && isExternal {
		return nil, fmt.Errorf("uma rota é proxy, redirect ou destino externo — nunca mais de um ao mesmo tempo")
	}
	if in.ViaLabels && (isRedirect || isExternal) {
		return nil, fmt.Errorf("rota via labels só funciona apontando pra um container (proxy) — não redirect nem destino externo")
	}

	switch {
	case isRedirect:
		if in.TargetContainer != "" || in.TargetPort != 0 {
			return nil, fmt.Errorf("uma rota é proxy OU redirect, não os dois")
		}
		if _, err := url.ParseRequestURI(in.RedirectTarget); err != nil {
			return nil, fmt.Errorf("URL de redirecionamento inválida")
		}
		if strings.ContainsAny(in.RedirectTarget, "\"\n\r") {
			return nil, fmt.Errorf("URL de redirecionamento contém caractere inválido")
		}
	case isExternal:
		if in.TargetContainer != "" || in.TargetPort != 0 {
			return nil, fmt.Errorf("uma rota é proxy pra container OU destino externo, não os dois")
		}
		parsed, err := url.ParseRequestURI(in.TargetURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, fmt.Errorf("URL de destino externo inválida — precisa ser http:// ou https://")
		}
		if strings.ContainsAny(in.TargetURL, "\"\n\r") {
			return nil, fmt.Errorf("URL de destino externo contém caractere inválido")
		}
		if isInternalHost(parsed.Hostname()) {
			return nil, fmt.Errorf("URL de destino externo não pode apontar pra endereço interno/privado")
		}
	default:
		if !containerNameRegex.MatchString(in.TargetContainer) {
			return nil, fmt.Errorf("nome de container inválido")
		}
		if in.TargetPort <= 0 || in.TargetPort > 65535 {
			return nil, fmt.Errorf("porta inválida")
		}
		// Modo via labels não conecta na nossa rede gestpg-managed — quem
		// precisa alcançar o alvo é o Traefik EXTERNO, não o gestpg-traefik;
		// essa conexão acontece depois, em applyLabelRoute, nas redes dele.
		if !in.ViaLabels {
			if err := s.docker.ConnectNetwork(ctx, s.networkName, in.TargetContainer); err != nil {
				return nil, fmt.Errorf("conectando %q à rede gerenciada: %w", in.TargetContainer, err)
			}
		}
	}

	var r ProxyRoute
	err := s.pool.QueryRow(ctx, `
		INSERT INTO proxy_routes (
			domain, target_container, target_port, target_url, tls,
			path_prefix, strip_prefix, redirect_target, redirect_permanent, https_redirect,
			via_labels, cert_resolver
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, domain, target_container, target_port, target_url, tls,
		          path_prefix, strip_prefix, redirect_target, redirect_permanent, https_redirect,
		          via_labels, cert_resolver, created_at
	`, in.Domain, in.TargetContainer, in.TargetPort, in.TargetURL, in.TLS,
		pathPrefix, in.StripPrefix, in.RedirectTarget, in.RedirectPermanent, in.HTTPSRedirect,
		in.ViaLabels, in.CertResolver,
	).Scan(
		&r.ID, &r.Domain, &r.TargetContainer, &r.TargetPort, &r.TargetURL, &r.TLS,
		&r.PathPrefix, &r.StripPrefix, &r.RedirectTarget, &r.RedirectPermanent, &r.HTTPSRedirect,
		&r.ViaLabels, &r.CertResolver, &r.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("salvando rota: %w", err)
	}

	var applyErr error
	if r.ViaLabels {
		applyErr = s.applyLabelRoute(ctx, r)
	} else {
		applyErr = writeDynamicConfig(r)
	}
	if applyErr != nil {
		_, _ = s.pool.Exec(ctx, `DELETE FROM proxy_routes WHERE id = $1`, r.ID)
		return nil, applyErr
	}
	return &r, nil
}

// applyLabelRoute recria SÓ o container alvo com os labels do Traefik —
// nunca toca no container do Traefik externo (EasyPanel etc). Best-effort:
// conecta o alvo em toda rede que o Traefik externo detectado já está, pra
// aumentar a chance dele alcançar o alvo mesmo sem saber a topologia de rede
// de quem administra esse Traefik.
func (s *Service) applyLabelRoute(ctx context.Context, r ProxyRoute) error {
	containerID, err := s.docker.FindContainerIDByName(ctx, r.TargetContainer)
	if err != nil {
		return fmt.Errorf("procurando container %q: %w", r.TargetContainer, err)
	}
	if containerID == "" {
		return fmt.Errorf("container %q não encontrado", r.TargetContainer)
	}

	if _, networks, found, err := s.docker.DetectExternalTraefik(ctx); err == nil && found {
		for _, netName := range networks {
			_ = s.docker.ConnectNetwork(ctx, netName, containerID)
		}
	}

	if _, err := s.docker.RecreateContainerWithLabels(ctx, containerID, buildTraefikLabels(r), "traefik."); err != nil {
		return fmt.Errorf("aplicando labels do traefik no container %q: %w", r.TargetContainer, err)
	}
	return nil
}

// buildTraefikLabels monta os labels no formato que o provider Docker do
// Traefik espera — mesma regra de roteamento que writeDynamicConfig gera
// pro file provider, só que como label em vez de YAML em disco.
func buildTraefikLabels(r ProxyRoute) map[string]string {
	name := routeSafeName(r.Domain)
	pathPrefix := r.PathPrefix
	if pathPrefix == "" {
		pathPrefix = "/"
	}
	rule := fmt.Sprintf("Host(`%s`)", r.Domain)
	if pathPrefix != "/" {
		rule += fmt.Sprintf(" && PathPrefix(`%s`)", pathPrefix)
	}

	entrypoints := "web"
	labels := map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.routers.%s.rule", name):                      rule,
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", name): strconv.Itoa(r.TargetPort),
	}
	if r.TLS {
		entrypoints = "web,websecure"
		if r.CertResolver != "" {
			labels[fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", name)] = r.CertResolver
		} else {
			labels[fmt.Sprintf("traefik.http.routers.%s.tls", name)] = "true"
		}
	}
	labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", name)] = entrypoints

	if r.StripPrefix && pathPrefix != "/" {
		mw := name + "-strip"
		labels[fmt.Sprintf("traefik.http.middlewares.%s.stripprefix.prefixes", mw)] = pathPrefix
		labels[fmt.Sprintf("traefik.http.routers.%s.middlewares", name)] = mw
	}
	return labels
}

func (s *Service) DeleteProxyRoute(ctx context.Context, id string) error {
	var domain, targetContainer string
	var viaLabels bool
	err := s.pool.QueryRow(ctx, `SELECT domain, target_container, via_labels FROM proxy_routes WHERE id = $1`, id).
		Scan(&domain, &targetContainer, &viaLabels)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("rota não encontrada")
		}
		return fmt.Errorf("lendo rota: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM proxy_routes WHERE id = $1`, id); err != nil {
		return fmt.Errorf("excluindo rota: %w", err)
	}
	if viaLabels {
		if targetContainer == "" {
			return nil
		}
		containerID, err := s.docker.FindContainerIDByName(ctx, targetContainer)
		if err != nil || containerID == "" {
			return nil
		}
		_, err = s.docker.RecreateContainerWithLabels(ctx, containerID, nil, "traefik.")
		if err != nil {
			return fmt.Errorf("removendo labels do traefik do container %q: %w", targetContainer, err)
		}
		return nil
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

// writeDynamicConfig monta o YAML do file provider do Traefik pra uma rota.
// Redirect-only usa o service builtin `noop@internal` (não tem backend de
// verdade, só middleware) — é o jeito documentado do Traefik de ter um
// router que só redireciona. https_redirect adiciona um SEGUNDO router no
// entrypoint "web" pro mesmo Host/path — sem isso, hoje (TLS=true sem essa
// opção) a porta 80 não tem router nenhum pra esse domínio e 404a; com a
// opção, ela redireciona pra https em vez de 404ar.
func writeDynamicConfig(r ProxyRoute) error {
	name := routeSafeName(r.Domain)
	pathPrefix := r.PathPrefix
	if pathPrefix == "" {
		pathPrefix = "/"
	}
	rule := fmt.Sprintf("Host(`%s`)", r.Domain)
	if pathPrefix != "/" {
		rule += fmt.Sprintf(" && PathPrefix(`%s`)", pathPrefix)
	}

	var b strings.Builder
	b.WriteString("http:\n")

	stripMiddleware := !r.isRedirect() && r.StripPrefix && pathPrefix != "/"
	httpsRedirectMiddleware := !r.isRedirect() && r.HTTPSRedirect && r.TLS

	if r.isRedirect() || stripMiddleware || httpsRedirectMiddleware {
		b.WriteString("  middlewares:\n")
		if r.isRedirect() {
			fmt.Fprintf(&b, "    %s-redirect:\n      redirectRegex:\n        regex: \"^.*\"\n        replacement: \"%s\"\n        permanent: %t\n",
				name, r.RedirectTarget, r.RedirectPermanent)
		}
		if stripMiddleware {
			fmt.Fprintf(&b, "    %s-strip:\n      stripPrefix:\n        prefixes:\n          - \"%s\"\n", name, pathPrefix)
		}
		if httpsRedirectMiddleware {
			fmt.Fprintf(&b, "    %s-tohttps:\n      redirectScheme:\n        scheme: https\n        permanent: true\n", name)
		}
	}

	b.WriteString("  routers:\n")

	if r.isRedirect() {
		entrypoints := []string{"web"}
		tlsBlock := ""
		if r.TLS {
			entrypoints = append(entrypoints, "websecure")
			tlsBlock = "\n      tls:\n        certResolver: le"
		}
		fmt.Fprintf(&b, "    %s:\n      rule: \"%s\"\n      entryPoints:\n", name, rule)
		for _, ep := range entrypoints {
			fmt.Fprintf(&b, "        - %s\n", ep)
		}
		fmt.Fprintf(&b, "      middlewares:\n        - %s-redirect\n      service: noop@internal%s\n", name, tlsBlock)
	} else {
		entrypoint := "web"
		tlsBlock := ""
		if r.TLS {
			entrypoint = "websecure"
			tlsBlock = "\n      tls:\n        certResolver: le"
		}
		fmt.Fprintf(&b, "    %s:\n      rule: \"%s\"\n      entryPoints:\n        - %s\n", name, rule, entrypoint)
		if stripMiddleware {
			fmt.Fprintf(&b, "      middlewares:\n        - %s-strip\n", name)
		}
		fmt.Fprintf(&b, "      service: %s%s\n", name, tlsBlock)

		if httpsRedirectMiddleware {
			fmt.Fprintf(&b, "    %s-web:\n      rule: \"%s\"\n      entryPoints:\n        - web\n      middlewares:\n        - %s-tohttps\n      service: noop@internal\n",
				name, rule, name)
		}

		fmt.Fprintf(&b, "  services:\n    %s:\n      loadBalancer:\n        servers:\n          - url: \"%s\"\n",
			name, r.upstreamURL())
	}

	return os.WriteFile(dynamicConfigPath(r.Domain), []byte(b.String()), 0o644)
}

func removeDynamicConfig(domain string) error {
	err := os.Remove(dynamicConfigPath(domain))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removendo config dinâmica: %w", err)
	}
	return nil
}
