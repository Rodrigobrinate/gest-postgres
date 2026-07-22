package docker

import (
	"os"
	"strconv"
	"strings"
)

// containerIOStatOps é o fallback pra contagem de operações de I/O quando a
// API de stats do Docker devolve 0 (host cgroup v2 — ver comentário em
// monitor.go). O kernel AINDA expõe operação por container em
// /sys/fs/cgroup/.../io.stat (campos rios=/wios=), só que o Docker não
// repassa isso pela API de stats — lê direto do cgroupfs bind-montado
// (/hostcgroup, ver docker-compose.yml).
//
// O caminho exato varia por driver de cgroup do Docker (systemd vs
// cgroupfs) — tenta os dois padrões mais comuns nessa ordem e usa o
// primeiro que existir. NÃO TESTADO AO VIVO ainda (precisa de um host real
// pra confirmar qual caminho bate) — se nenhum existir, devolve 0 igual o
// comportamento de antes, sem quebrar nada.
func containerIOStatOps(containerID string) (readOps, writeOps uint64, ok bool) {
	candidates := []string{
		"/hostcgroup/system.slice/docker-" + containerID + ".scope/io.stat", // driver systemd (default mais comum hoje)
		"/hostcgroup/docker/" + containerID + "/io.stat",                    // driver cgroupfs
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		r, w := parseIOStat(string(data))
		return r, w, true
	}
	return 0, 0, false
}

// parseIOStat lê o formato de /sys/fs/cgroup/.../io.stat (cgroup v2): uma
// linha por dispositivo de bloco, campos "chave=valor" separados por
// espaço, ex: "254:0 rbytes=123 wbytes=456 rios=7 wios=8 dbytes=0 dios=0".
// Soma entre dispositivos (container pode ter I/O em mais de um).
func parseIOStat(data string) (readOps, writeOps uint64) {
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		for _, f := range fields {
			if v, ok := strings.CutPrefix(f, "rios="); ok {
				if n, err := strconv.ParseUint(v, 10, 64); err == nil {
					readOps += n
				}
			}
			if v, ok := strings.CutPrefix(f, "wios="); ok {
				if n, err := strconv.ParseUint(v, 10, 64); err == nil {
					writeOps += n
				}
			}
		}
	}
	return readOps, writeOps
}
