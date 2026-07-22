package docker

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// hostCPUSample é a linha agregada "cpu " de /proc/stat — jiffies
// acumulados desde o boot, por categoria.
type hostCPUSample struct {
	idle  uint64 // idle + iowait: tempo que a CPU não tava fazendo trabalho útil
	total uint64 // soma de todas as categorias
}

var (
	hostCPUMu   sync.Mutex
	hostCPUPrev *hostCPUSample
)

// HostCPUPercent lê /proc/stat do HOST de verdade (bind mount /hostcpu, ver
// docker-compose.yml) — nunca soma de container, mesmo raciocínio de
// HostMemoryUsage (kernel/dockerd/sshd/cron/firewall-agent/update-agent
// nunca entram na soma de CPU por container).
//
// Diferente do disco/memória, CPU não dá pra ler num instante só — jiffies
// de /proc/stat são acumulados desde o boot, então "% em uso" precisa da
// DIFERENÇA entre duas leituras num intervalo. O stats de container por
// contêiner do Docker resolve isso devolvendo duas amostras (atual +
// anterior) numa chamada só; /proc/stat não tem equivalente, então essa
// função guarda a última amostra em memória (pacote-level, thread-safe) e
// compara com a atual a cada chamada — funciona porque o dashboard chama
// isso periodicamente (poll do frontend), não uma vez só. Primeira chamada
// depois do backend subir não tem amostra anterior pra comparar: devolve
// erro (quem chama trata como "ainda não disponível", cai pro fallback de
// soma de container só nessa primeira vez) em vez de inventar um 0%.
func HostCPUPercent() (float64, error) {
	sample, err := readHostCPUSample()
	if err != nil {
		return 0, err
	}

	hostCPUMu.Lock()
	prev := hostCPUPrev
	hostCPUPrev = sample
	hostCPUMu.Unlock()

	if prev == nil {
		return 0, fmt.Errorf("primeira leitura de CPU do host, sem amostra anterior pra comparar ainda")
	}

	totalDelta := sample.total - prev.total
	idleDelta := sample.idle - prev.idle
	if totalDelta == 0 {
		return 0, fmt.Errorf("nenhum tempo de CPU passou entre as duas leituras")
	}

	busy := float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	if busy < 0 {
		busy = 0
	}
	if busy > 100 {
		busy = 100
	}
	return busy, nil
}

func readHostCPUSample() (*hostCPUSample, error) {
	f, err := os.Open("/hostcpu")
	if err != nil {
		return nil, fmt.Errorf("lendo CPU do host (mount /hostcpu ausente?): %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return nil, fmt.Errorf("/hostcpu vazio")
	}
	line := scanner.Text()
	if !strings.HasPrefix(line, "cpu ") {
		return nil, fmt.Errorf("/hostcpu em formato inesperado")
	}

	fields := strings.Fields(line)[1:] // descarta o rótulo "cpu"
	var total uint64
	var idle uint64
	for i, raw := range fields {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			continue // campo extra desconhecido de kernel mais novo — ignora, não quebra
		}
		total += v
		// índices 3 (idle) e 4 (iowait) na ordem padrão de /proc/stat:
		// user nice system idle iowait irq softirq steal guest guest_nice
		if i == 3 || i == 4 {
			idle += v
		}
	}

	return &hostCPUSample{idle: idle, total: total}, nil
}
