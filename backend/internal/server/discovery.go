package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gest-postgres/backend/internal/docker"
)

type DiscoveredContainer struct {
	ContainerID string   `json:"container_id"`
	Name        string   `json:"name"`
	Image       string   `json:"image"`
	State       string   `json:"state"`
	Ports       []string `json:"ports"`
}

// ownComposeServices são os serviços do NOSSO próprio stack (docker-compose.yml)
// — nunca candidatos a "descoberta", mesmo o metadata-db batendo na heurística
// de imagem (é postgres:16-alpine de verdade, só que é NOSSO banco interno,
// não algo do usuário pra adotar).
var ownComposeServices = map[string]bool{
	"metadata-db":          true,
	"docker-socket-proxy":  true,
	"backend":              true,
	"frontend":             true,
}

// looksLikePostgres é a heurística de "isso parece um Postgres" — não tem
// como saber com certeza sem inspecionar o processo dentro do container.
// Olha o ÚLTIMO segmento do nome da imagem (sem tag) pra evitar falso
// positivo tipo "gest-postgres-frontend" batendo em "contains postgres".
func looksLikePostgres(image string, ports []string) bool {
	repo := strings.ToLower(image)
	if idx := strings.LastIndex(repo, ":"); idx > strings.LastIndex(repo, "/") {
		repo = repo[:idx]
	}
	segments := strings.Split(repo, "/")
	last := segments[len(segments)-1]

	nameMatches := last == "postgres" || last == "postgresql" ||
		strings.Contains(last, "pgvector") || strings.Contains(last, "timescaledb") ||
		strings.HasPrefix(last, "postgres-") || strings.HasSuffix(last, "-postgres") ||
		strings.HasPrefix(last, "postgresql-") || strings.HasSuffix(last, "-postgresql")
	if nameMatches {
		return true
	}
	for _, p := range ports {
		if strings.Contains(p, "5432") {
			return true
		}
	}
	return false
}

// DiscoverContainers lista containers Docker no host que parecem rodar
// Postgres e ainda não estão cadastrados na plataforma — é só isso que dá
// pra enxergar com a arquitetura atual (backend só fala com o Docker via
// socket-proxy, sem acesso ao host além disso). Postgres instalado nativo
// (fora de container) fica fora de alcance.
func (s *Service) DiscoverContainers(ctx context.Context) ([]DiscoveredContainer, error) {
	all, err := s.docker.ListAllContainers(ctx)
	if err != nil {
		return nil, err
	}

	known, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	knownIDs := make(map[string]bool, len(known))
	for _, k := range known {
		if k.ContainerID != "" {
			knownIDs[k.ContainerID] = true
		}
	}

	out := make([]DiscoveredContainer, 0)
	for _, c := range all {
		if knownIDs[c.ID] {
			continue
		}
		if c.Labels[docker.LabelManaged] == "true" {
			continue
		}
		if ownComposeServices[c.Labels["com.docker.compose.service"]] {
			continue
		}
		ports := make([]string, 0, len(c.Ports))
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type))
			} else {
				ports = append(ports, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
			}
		}
		if !looksLikePostgres(c.Image, ports) {
			continue
		}
		name := c.ID[:12]
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		out = append(out, DiscoveredContainer{
			ContainerID: c.ID,
			Name:        name,
			Image:       c.Image,
			State:       c.State,
			Ports:       ports,
		})
	}
	return out, nil
}

type RegisterDiscoveredInput struct {
	Name         string `json:"name"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	DatabaseName string `json:"database_name"`
}

// RegisterDiscovered "adota" um container Postgres que já existia fora da
// plataforma: não cria container nem volume novo, só valida que as
// credenciais informadas realmente conectam e passa a gerenciar o que já tá
// rodando. Conecta o container na rede gestpg-managed se ele ainda não
// estiver nela — é assim que o backend consegue falar com ele pelo nome
// depois (mesmo esquema usado pra tudo que a própria plataforma cria).
func (s *Service) RegisterDiscovered(ctx context.Context, containerID string, in RegisterDiscoveredInput) (*Server, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, fmt.Errorf("%w: nome é obrigatório", ErrValidation)
	}
	if in.Username == "" || in.Password == "" || in.DatabaseName == "" {
		return nil, fmt.Errorf("%w: usuário, senha e banco são obrigatórios", ErrValidation)
	}

	info, err := s.docker.InspectContainer(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("%w: container não encontrado: %v", ErrValidation, err)
	}
	if !info.Running {
		return nil, fmt.Errorf("%w: container precisa estar rodando pra validar as credenciais", ErrValidation)
	}

	detail, err := s.docker.InspectForDiscovery(ctx, containerID)
	if err != nil {
		return nil, err
	}

	onManagedNetwork := false
	for _, n := range detail.Networks {
		if n == s.networkName {
			onManagedNetwork = true
			break
		}
	}
	if !onManagedNetwork {
		if err := s.docker.ConnectNetwork(ctx, s.networkName, containerID); err != nil {
			return nil, fmt.Errorf("conectando container à rede gerenciada: %w", err)
		}
	}

	// Confirma de verdade que as credenciais funcionam antes de salvar
	// qualquer coisa — sem isso um cadastro com senha errada ficaria
	// registrado mas inutilizável.
	if err := waitPostgresReady(ctx, detail.Name, in.Username, in.Password, in.DatabaseName, 8*time.Second); err != nil {
		return nil, fmt.Errorf("%w: não conectou com as credenciais informadas: %v", ErrValidation, err)
	}

	encryptedPassword, err := s.secretBox.Seal(in.Password)
	if err != nil {
		return nil, fmt.Errorf("cifrando senha: %w", err)
	}

	hostPort := detail.HostPort
	if hostPort == 0 {
		// Container não publica a porta 5432 pro host — reserva um slot só
		// pra satisfazer a constraint UNIQUE do metadata DB; a plataforma
		// sempre fala com ele pelo nome dentro da rede Docker mesmo, então
		// isso não afeta nenhuma função interna, só a connection string
		// externa (que não vai funcionar até o container ser recriado com
		// a porta publicada — fora do que dá pra fazer sem recriar).
		hostPort, err = s.allocatePort(ctx)
		if err != nil {
			return nil, err
		}
	}

	status := StatusStopped
	if info.Running {
		status = StatusRunning
	}

	record := &Server{
		Name:              in.Name,
		Description:       "descoberto automaticamente (container pré-existente)",
		Version:           "desconhecida",
		Status:            status,
		Preset:            PresetCustom,
		Resources:         Resources{CPUCores: 1, MemoryMB: 1024, DiskGB: 10},
		Config:            ConfigForResources(Resources{CPUCores: 1, MemoryMB: 1024, DiskGB: 10}),
		HostPort:          hostPort,
		Username:          in.Username,
		PasswordEncrypted: encryptedPassword,
		DatabaseName:      in.DatabaseName,
		ContainerID:       containerID,
		ContainerName:     detail.Name,
		VolumeName:        detail.VolumeName,
	}

	if err := s.repo.Create(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}
