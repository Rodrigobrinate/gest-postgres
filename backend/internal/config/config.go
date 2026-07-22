// Package config carrega configuração do backend a partir de variáveis de ambiente.
package config

import (
	"fmt"
	"net"
	"os"
	"strings"
)

type Config struct {
	// HTTPAddr é o endereço em que a API escuta, ex: ":28080".
	HTTPAddr string

	// MetadataDatabaseURL é a connection string do Postgres de metadados da plataforma
	// (não confundir com os Postgres gerenciados que os usuários criam).
	MetadataDatabaseURL string

	// DockerHost é o endpoint do Docker Engine API. Em produção deve apontar para o
	// docker-socket-proxy, nunca para o socket direto.
	DockerHost string

	// ManagedNetworkName é a rede Docker compartilhada onde os containers Postgres
	// gerenciados são conectados.
	ManagedNetworkName string

	// CredentialEncryptionKey é usada para cifrar (AES-GCM) as senhas dos servidores
	// gerenciados antes de gravar no banco de metadados. Deve ter 32 bytes em hex (64 chars).
	CredentialEncryptionKey string

	// ManagedPortRangeStart/End definem a faixa de portas do host usada para expor
	// os containers Postgres gerenciados.
	ManagedPortRangeStart int
	ManagedPortRangeEnd   int

	// AdminPassword semeia o único usuário admin na primeira subida (ver
	// internal/auth.SeedAdminIfMissing). Se vazia, uma senha aleatória é
	// gerada e logada uma vez só — nunca sobe sem login nenhum.
	AdminPassword string

	// AllowedOrigins é a allowlist de CORS — nunca reflete Origin
	// incondicionalmente (isso libera qualquer site a fazer request
	// cross-origin com o cookie de sessão anexado). Populada pelo setup.sh
	// com o IP/domínio público detectado; setável à mão via ALLOWED_ORIGINS
	// (lista separada por vírgula) se o admin trocar de domínio depois.
	AllowedOrigins []string

	// TrustedProxies são os CIDRs (ou IPs soltos, tratados como /32 ou /128)
	// cujo X-Forwarded-For/X-Forwarded-Proto é honrado — vazio por padrão
	// (instalação padrão publica a porta do backend crua, sem proxy na
	// frente, então RemoteAddr já É o peer real; confiar em X-Forwarded-*
	// incondicionalmente deixa qualquer requisição direta forjar o header e
	// burlar throttle de login por IP). Só popular se um reverse proxy de
	// verdade (Traefik, por exemplo) estiver na frente do backend — nesse
	// caso, o CIDR da rede Docker onde ele roda (ex: a sub-rede de
	// gestpg-internal).
	TrustedProxies []*net.IPNet

	// CloudflareTunnelToken/IntegrationKeySeed: modo de conexão com o
	// sistema mestre na Cloudflare (setup.sh --cloud-token/--integration-key,
	// gravados em CLOUDFLARE_TUNNEL_TOKEN/INTEGRATION_KEY_SEED no .env).
	// Vazios por padrão — lidos uma vez no boot pra subir o cloudflared e
	// semear a chave de integração sozinho (ver cmd/api/main.go), mesmo
	// canal de confiança que ADMIN_PASSWORD já usa.
	CloudflareTunnelToken string
	IntegrationKeySeed    string
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:                getEnv("HTTP_ADDR", ":28080"),
		MetadataDatabaseURL:     getEnv("METADATA_DATABASE_URL", ""),
		DockerHost:              getEnv("DOCKER_HOST", "tcp://docker-socket-proxy:2375"),
		ManagedNetworkName:      getEnv("MANAGED_NETWORK_NAME", "gestpg-managed"),
		CredentialEncryptionKey: getEnv("CREDENTIAL_ENCRYPTION_KEY", ""),
		ManagedPortRangeStart:   55432,
		ManagedPortRangeEnd:     56432,
		AdminPassword:           getEnv("ADMIN_PASSWORD", ""),
		AllowedOrigins:          splitCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:4173")),
		CloudflareTunnelToken:   getEnv("CLOUDFLARE_TUNNEL_TOKEN", ""),
		IntegrationKeySeed:      getEnv("INTEGRATION_KEY_SEED", ""),
	}

	trustedProxies, err := parseCIDRList(splitCSV(getEnv("TRUSTED_PROXIES", "")))
	if err != nil {
		return nil, fmt.Errorf("TRUSTED_PROXIES inválida: %w", err)
	}
	cfg.TrustedProxies = trustedProxies

	if cfg.MetadataDatabaseURL == "" {
		return nil, fmt.Errorf("METADATA_DATABASE_URL não configurada")
	}
	if len(cfg.CredentialEncryptionKey) != 64 {
		return nil, fmt.Errorf("CREDENTIAL_ENCRYPTION_KEY deve ter 64 caracteres hex (32 bytes)")
	}
	// A validação de comprimento sozinha aceita o placeholder do
	// .env.example (64 zeros) — chave AES conhecida por qualquer leitor do
	// repositório. setup.sh regenera isso na instalação normal, mas nada no
	// código impedia um deploy manual (ex: `cp .env.example .env` sem
	// passar pelo setup.sh) de subir com ela.
	if isAllZeroHex(cfg.CredentialEncryptionKey) {
		return nil, fmt.Errorf("CREDENTIAL_ENCRYPTION_KEY está no valor placeholder (todo zero) — gere uma de verdade com: openssl rand -hex 32")
	}

	return cfg, nil
}

func isAllZeroHex(s string) bool {
	for _, c := range s {
		if c != '0' {
			return false
		}
	}
	return true
}

// parseCIDRList aceita tanto CIDR ("10.0.0.0/16") quanto IP solto
// ("172.20.0.5", tratado como /32 ou /128).
func parseCIDRList(entries []string) ([]*net.IPNet, error) {
	nets := make([]*net.IPNet, 0, len(entries))
	for _, entry := range entries {
		if strings.Contains(entry, "/") {
			_, ipNet, err := net.ParseCIDR(entry)
			if err != nil {
				return nil, fmt.Errorf("%q: %w", entry, err)
			}
			nets = append(nets, ipNet)
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			return nil, fmt.Errorf("%q não é um IP nem CIDR válido", entry)
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return nets, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
