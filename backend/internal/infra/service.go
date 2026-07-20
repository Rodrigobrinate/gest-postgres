// Package infra cobre gestão de Docker que vai além de "servidor Postgres
// gerenciado" (o domínio do package server): containers/networks/volumes
// genéricos, deploy via compose/Dockerfile, Traefik e firewall do host.
package infra

import (
	"github.com/gest-postgres/backend/internal/crypto"
	"github.com/gest-postgres/backend/internal/docker"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	docker *docker.Client
	pool   *pgxpool.Pool

	networkName        string
	secretBox          *crypto.SecretBox
	containerHistories *containerHistories
}

func NewService(dockerClient *docker.Client, pool *pgxpool.Pool, networkName string, secretBox *crypto.SecretBox) *Service {
	return &Service{
		docker:             dockerClient,
		pool:               pool,
		networkName:        networkName,
		secretBox:          secretBox,
		containerHistories: newContainerHistories(),
	}
}
