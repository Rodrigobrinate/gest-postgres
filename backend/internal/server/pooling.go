package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gest-postgres/backend/internal/docker"
)

var allowedPoolModes = map[string]bool{
	"session":     true,
	"transaction": true,
	"statement":   true,
}

// EnablePooling sobe um container pgbouncer companheiro apontando pro
// Postgres gerenciado, publicado numa porta própria (a aplicação cliente
// troca a porta do Postgres pela do pooler, nada muda do lado do banco).
// Precisa do container Postgres já existente e rodando — pgbouncer com
// AUTH_TYPE=md5 valida a senha na hora da conexão, então se o Postgres não
// tiver disponível ainda a criação segue (o container sobe e fica tentando),
// mas exigir "running" aqui evita um pooler apontando pro nada por engano.
func (s *Service) EnablePooling(ctx context.Context, id, poolMode string) (*Server, error) {
	if poolMode == "" {
		poolMode = "transaction"
	}
	if !allowedPoolModes[poolMode] {
		return nil, fmt.Errorf("%w: pool_mode %q inválido", ErrValidation, poolMode)
	}

	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.PoolerEnabled {
		return nil, fmt.Errorf("%w: pooling já está habilitado nesse servidor", ErrValidation)
	}
	if record.ContainerID == "" || record.Status != StatusRunning {
		return nil, fmt.Errorf("%w: servidor precisa estar rodando pra habilitar o pooling", ErrValidation)
	}

	password, err := s.secretBox.Open(record.PasswordEncrypted)
	if err != nil {
		return nil, err
	}

	hostPort, err := s.allocatePort(ctx)
	if err != nil {
		return nil, err
	}

	poolerName := record.ContainerName + "-pgbouncer"

	if err := s.docker.PullImageIfMissing(ctx, "edoburu/pgbouncer:latest"); err != nil {
		return nil, fmt.Errorf("baixando imagem do pgbouncer: %w", err)
	}

	containerID, err := s.docker.CreatePgBouncerContainer(ctx, docker.CreatePgBouncerInput{
		Name:         poolerName,
		TargetHost:   record.ContainerName,
		Username:     record.Username,
		Password:     password,
		DatabaseName: record.DatabaseName,
		PoolMode:     poolMode,
		HostPort:     hostPort,
		NetworkName:  s.networkName,
		ServerID:     id,
	})
	if err != nil {
		return nil, fmt.Errorf("criando container pgbouncer: %w", err)
	}

	if err := s.docker.WaitHealthy(ctx, containerID, 30*time.Second); err != nil {
		_ = s.docker.RemoveContainer(ctx, containerID, "", false)
		return nil, fmt.Errorf("pgbouncer não subiu a tempo: %w", err)
	}

	if err := s.repo.SetPooler(ctx, id, containerID, poolerName, hostPort, poolMode); err != nil {
		_ = s.docker.RemoveContainer(ctx, containerID, "", false)
		return nil, err
	}

	return s.repo.Get(ctx, id)
}

// recreatePooler troca o container pgbouncer por um novo com a senha
// atualizada — a imagem gera userlist.txt a partir da env var DB_PASSWORD no
// boot, então não tem como só "avisar" o container já rodando de uma senha
// nova, precisa recriar. Chamada depois de uma rotação de senha do superuser
// pra evitar o pooler ficar autenticando com credencial velha silenciosamente
// (mesma classe de bug que a rotação de senha principal já teve).
func (s *Service) recreatePooler(ctx context.Context, record *Server, newPassword string) error {
	if err := s.docker.RemoveContainer(ctx, record.PoolerContainerID, "", false); err != nil {
		return fmt.Errorf("removendo pgbouncer antigo: %w", err)
	}
	containerID, err := s.docker.CreatePgBouncerContainer(ctx, docker.CreatePgBouncerInput{
		Name:         record.PoolerContainerName,
		TargetHost:   record.ContainerName,
		Username:     record.Username,
		Password:     newPassword,
		DatabaseName: record.DatabaseName,
		PoolMode:     record.PoolerPoolMode,
		HostPort:     record.PoolerHostPort,
		NetworkName:  s.networkName,
		ServerID:     record.ID,
	})
	if err != nil {
		return fmt.Errorf("recriando pgbouncer: %w", err)
	}
	return s.repo.SetPooler(ctx, record.ID, containerID, record.PoolerContainerName, record.PoolerHostPort, record.PoolerPoolMode)
}

// DisablePooling remove o container pgbouncer e limpa o registro — o
// Postgres em si nunca é tocado, só o companheiro de pooling.
func (s *Service) DisablePooling(ctx context.Context, id string) error {
	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if !record.PoolerEnabled {
		return nil
	}
	if record.PoolerContainerID != "" {
		if err := s.docker.RemoveContainer(ctx, record.PoolerContainerID, "", false); err != nil && !strings.Contains(err.Error(), "No such container") {
			return err
		}
	}
	return s.repo.ClearPooler(ctx, id)
}
