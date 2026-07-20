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
func HostDiskUsage() (totalBytes, usedBytes, freeBytes int64, err error) {
	var stat unix.Statfs_t
	if err := unix.Statfs("/hostfs", &stat); err != nil {
		return 0, 0, 0, fmt.Errorf("lendo disco do host (mount /hostfs ausente?): %w", err)
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)
	return total, total - free, free, nil
}
