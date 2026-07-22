package docker

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// HostDiskUsage lê o filesystem do HOST de verdade via statfs — não é
// possível pela API do Docker (nem com a categoria SYSTEM, que só dá o que o
// Docker ocupa: imagens/containers/volumes, não o disco todo). Só funciona
// se o host montar algum diretório seu dentro do container do backend em
// /hostfs (ver docker-compose.yml) — sem esse mount, retorna erro e quem
// chama trata como "não disponível" em vez de quebrar.
//
// Monta só um arquivo único (/etc/hostname, não /etc inteiro nem a raiz) de
// propósito: statfs só precisa de UM caminho que esteja no mesmo filesystem
// que se quer medir — não importa se é arquivo ou diretório — e num droplet
// de disco único (o cenário do setup.sh) qualquer arquivo de /etc está no
// mesmo filesystem que /. Um arquivo isolado dá o número certo sem expor
// shadow/ssh/sudoers/cron (o resto de /etc) de leitura dentro do container.
// total/usado/livre seguem a MESMA convenção do `df` (e da maioria dos
// painéis, incluindo o que motivou essa correção — comparado ao vivo
// contra o Painel do EasyPanel no mesmo host, números não batiam): "livre"
// é Bavail (espaço que um processo comum consegue usar, já excluindo a
// reserva do ext4 pra root — tipicamente 5% do filesystem), "usado" é
// Blocks-Bfree (raw, sem excluir a reserva), e "total" é livre+usado — não
// Blocks*Bsize direto. A diferença prática: Blocks*Bsize conta a reserva
// do root como parte do "total" mas NUNCA como "usado" nem "livre" (ela não
// é nenhum dos dois pra um processo comum), o que inflava usado% sem
// nenhum uso real acontecer — exatamente o gap visto contra o EasyPanel.
func HostDiskUsage() (totalBytes, usedBytes, freeBytes int64, err error) {
	var stat unix.Statfs_t
	if err := unix.Statfs("/hostfs", &stat); err != nil {
		return 0, 0, 0, fmt.Errorf("lendo disco do host (mount /hostfs ausente?): %w", err)
	}
	bsize := int64(stat.Bsize)
	free := int64(stat.Bavail) * bsize
	used := (int64(stat.Blocks) - int64(stat.Bfree)) * bsize
	return free + used, used, free, nil
}
