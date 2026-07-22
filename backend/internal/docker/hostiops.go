package docker

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// wholeDiskRegex separa disco inteiro de partição/dispositivo virtual —
// /proc/diskstats lista os dois juntos, e somar ambos contaria a mesma
// operação duas vezes (I/O de partição é subconjunto do disco inteiro que a
// contém). Cobre os padrões de nome mais comuns em droplet de nuvem
// (virtio/scsi "vda"/"sda", Xen "xvda", NVMe "nvme0n1", eMMC "mmcblk0") —
// disco inteiro termina em letra (ou "n<N>"/"mmcblk<N>" sem sufixo de
// partição), partição sempre tem um dígito/"p<N>" a mais no final. Não
// cobre RAID (md*)/LVM (dm-*) de propósito — mesmo escopo "droplet de disco
// único" já assumido pela medição de disco via statfs (/hostfs).
var wholeDiskRegex = regexp.MustCompile(`^(sd[a-z]+|xvd[a-z]+|vd[a-z]+|hd[a-z]+)$|^nvme\d+n\d+$|^mmcblk\d+$`)

type hostIOPSSample struct {
	reads  uint64
	writes uint64
}

var (
	hostIOPSMu   sync.Mutex
	hostIOPSPrev *hostIOPSSample
	hostIOPSPrevAt time.Time // quando a amostra anterior foi lida — IOPS precisa do tempo real decorrido pra virar taxa (diferente de CPU%, que é uma razão adimensional e cancela o intervalo sozinha)
)

// HostIOPS lê /proc/diskstats do HOST de verdade (bind mount /hostdiskstats,
// ver docker-compose.yml) e devolve operações de leitura/escrita
// COMPLETADAS por segundo — não bytes, contagem de operação, pedido
// explícito comparando com um painel Zabbix noutro servidor. Mesmo
// raciocínio de HostCPUPercent: contador acumulado desde o boot, precisa de
// duas leituras pra virar taxa, guarda a amostra anterior em memória
// (pacote-level, thread-safe). Primeira chamada depois do backend subir
// devolve erro de propósito (sem amostra anterior pra comparar ainda).
func HostIOPS() (readOpsPerSec, writeOpsPerSec float64, err error) {
	sample, err := readHostIOPSSample()
	if err != nil {
		return 0, 0, err
	}
	now := time.Now()

	hostIOPSMu.Lock()
	prev := hostIOPSPrev
	prevAt := hostIOPSPrevAt
	hostIOPSPrev = sample
	hostIOPSPrevAt = now
	hostIOPSMu.Unlock()

	if prev == nil {
		return 0, 0, fmt.Errorf("primeira leitura de I/O do host, sem amostra anterior pra comparar ainda")
	}

	dt := now.Sub(prevAt).Seconds()
	if dt <= 0 {
		return 0, 0, fmt.Errorf("nenhum tempo passou entre as duas leituras")
	}

	readOps := float64(sample.reads-prev.reads) / dt
	writeOps := float64(sample.writes-prev.writes) / dt
	if readOps < 0 {
		readOps = 0
	}
	if writeOps < 0 {
		writeOps = 0
	}
	return readOps, writeOps, nil
}

func readHostIOPSSample() (*hostIOPSSample, error) {
	f, err := os.Open("/hostdiskstats")
	if err != nil {
		return nil, fmt.Errorf("lendo I/O do host (mount /hostdiskstats ausente?): %w", err)
	}
	defer f.Close()

	var totalReads, totalWrites uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 {
			continue
		}
		name := fields[2]
		if !wholeDiskRegex.MatchString(name) {
			continue
		}
		reads, err := strconv.ParseUint(fields[3], 10, 64)
		if err != nil {
			continue
		}
		writes, err := strconv.ParseUint(fields[7], 10, 64)
		if err != nil {
			continue
		}
		totalReads += reads
		totalWrites += writes
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("lendo /hostdiskstats: %w", err)
	}

	return &hostIOPSSample{reads: totalReads, writes: totalWrites}, nil
}
