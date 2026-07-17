// Package config carrega configuração do backend a partir de variáveis de ambiente.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	// HTTPAddr é o endereço em que a API escuta, ex: ":8080".
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
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:                getEnv("HTTP_ADDR", ":8080"),
		MetadataDatabaseURL:     getEnv("METADATA_DATABASE_URL", ""),
		DockerHost:              getEnv("DOCKER_HOST", "tcp://docker-socket-proxy:2375"),
		ManagedNetworkName:      getEnv("MANAGED_NETWORK_NAME", "gestpg-managed"),
		CredentialEncryptionKey: getEnv("CREDENTIAL_ENCRYPTION_KEY", ""),
		ManagedPortRangeStart:   55432,
		ManagedPortRangeEnd:     56432,
	}

	if cfg.MetadataDatabaseURL == "" {
		return nil, fmt.Errorf("METADATA_DATABASE_URL não configurada")
	}
	if len(cfg.CredentialEncryptionKey) != 64 {
		return nil, fmt.Errorf("CREDENTIAL_ENCRYPTION_KEY deve ter 64 caracteres hex (32 bytes)")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
