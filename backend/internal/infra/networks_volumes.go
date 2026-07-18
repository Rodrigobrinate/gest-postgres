package infra

import (
	"context"
	"fmt"
	"strings"

	"github.com/gest-postgres/backend/internal/docker"
)

// protectedNetworks nunca podem ser removidas por essa tela genérica —
// são as redes fixas da própria plataforma (ver docker-compose.yml raiz) +
// as builtin do Docker, que nem fazem sentido remover.
var protectedNetworks = map[string]bool{
	"bridge": true, "host": true, "none": true,
	"gestpg-internal": true, "gestpg-managed": true,
}

func (s *Service) ListNetworks(ctx context.Context) ([]docker.NetworkSummary, error) {
	return s.docker.ListNetworks(ctx)
}

func (s *Service) CreateNetwork(ctx context.Context, name string) error {
	_, err := s.docker.CreateNetwork(ctx, name)
	return err
}

func (s *Service) RemoveNetwork(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("id da rede é obrigatório")
	}
	nets, err := s.docker.ListNetworks(ctx)
	if err != nil {
		return err
	}
	// HasPrefix (não ==) porque IDs podem chegar truncados (o `docker` CLI
	// mostra só os 12 primeiros caracteres por padrão, e nada aqui garante
	// que quem chama sempre manda o ID completo de 64 caracteres que a
	// Engine API devolve).
	for _, n := range nets {
		if strings.HasPrefix(n.ID, id) && protectedNetworks[n.Name] {
			return fmt.Errorf("rede %q é da própria plataforma, não pode ser removida por aqui", n.Name)
		}
	}
	return s.docker.RemoveNetwork(ctx, id)
}

func (s *Service) ListVolumes(ctx context.Context) ([]docker.VolumeSummary, error) {
	return s.docker.ListVolumes(ctx)
}

func (s *Service) CreateVolume(ctx context.Context, name string) error {
	return s.docker.CreateVolume(ctx, name)
}

// RemoveVolume recusa apagar o volume de metadados da plataforma, o de
// backups, ou o volume de dados de QUALQUER servidor Postgres gerenciado —
// pra esses últimos existe um fluxo próprio (excluir o servidor, com sua
// própria confirmação de "apagar volume também"), não faz sentido essa tela
// genérica conseguir apagar o dado de um servidor por baixo dos panos.
func (s *Service) RemoveVolume(ctx context.Context, name string) error {
	if name == "gest-postgres_metadata_db_data" || name == "gest-postgres_backups_data" {
		return fmt.Errorf("volume %q é da própria plataforma, não pode ser removido por aqui", name)
	}
	var isManagedServerVolume bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM servers WHERE volume_name = $1)`, name).Scan(&isManagedServerVolume)
	if err != nil {
		return fmt.Errorf("checando se o volume pertence a um servidor gerenciado: %w", err)
	}
	if isManagedServerVolume {
		return fmt.Errorf("volume %q pertence a um servidor gerenciado — exclua o servidor pela tela de Servidores em vez disso", name)
	}
	return s.docker.RemoveVolume(ctx, name, false)
}
