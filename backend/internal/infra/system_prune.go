package infra

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// SystemPrune limpa "lixo" do Docker: containers parados, redes sem uso,
// imagens dangling (sem tag) e cache de build — via `docker system prune`
// (mesmo raciocínio de compose.go/build.go, shell out em vez de reimplementar
// via Engine API). De propósito SEM `--volumes` nem `-a`: volume nomeado
// nunca é tocado por esse botão (um servidor gerenciado parado ainda tem seu
// volume de dados considerado "sem uso" pelo Docker — apagar isso por engano
// seria catastrófico), e `-a` apagaria imagem sem tag mas ainda referenciada
// por histórico de build, não só lixo de verdade.
func (s *Service) SystemPrune(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "system", "prune", "-f")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker system prune falhou: %s", out.String())
	}
	return out.String(), nil
}
