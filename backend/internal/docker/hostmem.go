package docker

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// HostMemoryUsage lê /proc/meminfo do HOST de verdade (bind mount
// /hostmem, ver docker-compose.yml) — nunca soma de container. Existe
// porque somar MemoryUsedMB de cada container Docker sempre fica abaixo do
// uso real do host: kernel, dockerd/containerd, sshd, cron, o próprio
// firewall-agent/update-agent (rodam no host, não em container), e
// qualquer outra coisa fora de um cgroup Docker nunca entram nessa soma
// (achado comparando com o Painel do EasyPanel no mesmo host, que lê o
// host de verdade e mostrava bem mais memória "em uso" que o dashboard
// desta plataforma).
//
// "Usado" = MemTotal - MemAvailable (não MemTotal - MemFree). MemAvailable
// é o kernel estimando quanto dá pra usar sem trocar pra swap, já contando
// cache/buffer reclamável como "disponível" — é o mesmo cálculo que
// `free -m` e a maioria das ferramentas de monitoramento usam pra "% em
// uso"; MemFree sozinho conta cache de página como "usado", o que deixa a
// barra de uso artificialmente alta (praticamente sempre perto de 100% em
// qualquer Linux com uptime, já que o kernel usa RAM livre pra cache de
// disco de propósito).
func HostMemoryUsage() (totalBytes, usedBytes int64, err error) {
	f, err := os.Open("/hostmem")
	if err != nil {
		return 0, 0, fmt.Errorf("lendo memória do host (mount /hostmem ausente?): %w", err)
	}
	defer f.Close()

	var totalKB, availableKB int64
	var haveTotal, haveAvailable bool

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			totalKB, haveTotal = parseMeminfoLine(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			availableKB, haveAvailable = parseMeminfoLine(line)
		}
		if haveTotal && haveAvailable {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("lendo /hostmem: %w", err)
	}
	if !haveTotal || !haveAvailable {
		return 0, 0, fmt.Errorf("/hostmem sem MemTotal/MemAvailable — formato inesperado")
	}

	total := totalKB * 1024
	used := (totalKB - availableKB) * 1024
	return total, used, nil
}

// parseMeminfoLine extrai o valor em kB de uma linha tipo
// "MemTotal:       3915000 kB".
func parseMeminfoLine(line string) (int64, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, false
	}
	v, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
