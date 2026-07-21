// Comando update-agent roda DIRETO no host (nunca em container), mesmo
// espírito do firewall-agent: escuta só num socket Unix local
// (/run/gestpg-update.sock), o backend fala com ele via bind mount.
//
// Superfície proposital MÍNIMA: dois endpoints. GET /status devolve o
// resultado da última execução (nunca aceita parâmetro). POST /apply não
// aceita corpo nenhum — dispara sempre a MESMA pipeline fixa (`git pull` no
// repositório configurado em build/install time via GESTPG_REPO_DIR + o
// `./setup.sh` desse repositório), nunca um comando arbitrário vindo da
// rede. Não existe endpoint pra rodar comando livre — isso teria que ser uma
// decisão de arquitetura totalmente diferente (e muito mais perigosa).
//
// O pulo do gato: o próprio `setup.sh` reinstala e reinicia ESTE agente a
// cada execução (mesmo padrão do firewall-agent, pra pegar código novo do
// agente sem depender de reboot manual). Se a pipeline disparada por
// POST /apply rodasse como filho direto deste processo, `systemctl restart
// gestpg-update-agent` (chamado pelo PRÓPRIO `setup.sh` que a pipeline tá
// executando) mataria a pipeline no meio do caminho — porque o systemd mata
// todo processo do cgroup do serviço ao reiniciar, não só o PID principal.
// Por isso a pipeline roda numa unit systemd TRANSIENTE própria
// (`systemd-run --unit=...`), num cgroup separado do nosso — sobrevive
// tranquilamente a este agente reiniciar (ou até cair) no meio do processo.
// Estado (rodando/sucesso/falha) e log ficam em arquivo em disco, nunca em
// memória, pelo mesmo motivo: sobrevivem ao agente reiniciar.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	socketPath = "/run/gestpg-update.sock"
	logPath    = "/var/log/gestpg-update.log"
	statePath  = "/var/log/gestpg-update-state.json"
	maxLogLines = 400
)

// updateScript é uma pipeline FIXA, sem interpolação de string nenhuma —
// tudo que varia (caminho do repo, dos arquivos de estado/log) chega via
// variável de ambiente que o systemd-run passa pro processo, nunca
// concatenado no texto do script. Isso fecha qualquer superfície de
// injeção de comando: não existe dado vindo da requisição HTTP que chegue
// perto de um shell.
const updateScript = `
echo "=== update iniciado $(date -Iseconds) ==="
cd "$REPO_DIR" || exit 90
git config --global --get-all safe.directory | grep -qxF "$REPO_DIR" || git config --global --add safe.directory "$REPO_DIR"
git pull && ./setup.sh
ec=$?
echo "=== update finalizado, exit code $ec ==="
if [ "$ec" -eq 0 ]; then st=success; else st=failed; fi
printf '{"status":"%s","finished_at":"%s","exit_code":%d,"unit_name":"%s"}\n' "$st" "$(date -Iseconds)" "$ec" "$UNIT_NAME" > "$STATE_PATH"
`

type State struct {
	Status     string `json:"status"` // idle | running | success | failed | unknown
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	Error      string `json:"error,omitempty"`
	UnitName   string `json:"unit_name,omitempty"`
	LogTail    string `json:"log_tail,omitempty"`
}

func main() {
	if os.Getenv("GESTPG_REPO_DIR") == "" {
		log.Fatal("GESTPG_REPO_DIR não configurada — setup.sh deveria ter passado isso na unit systemd")
	}

	os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("escutando socket %s: %v", socketPath, err)
	}
	// 0660, não 0666 — mesmo raciocínio do firewall-agent (ver comentário
	// lá): este processo e o backend rodam como root, então 0660 já basta
	// pro backend conectar; 0666 deixaria qualquer usuário local do host
	// disparar `git pull && ./setup.sh` como root, o que é bem pior aqui do
	// que no firewall-agent.
	if err := os.Chmod(socketPath, 0o660); err != nil {
		log.Fatalf("ajustando permissão do socket: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", handleStatus)
	mux.HandleFunc("POST /apply", handleApply)

	log.Printf("update-agent escutando em %s (repo: %s)", socketPath, os.Getenv("GESTPG_REPO_DIR"))
	log.Fatal(http.Serve(listener, mux))
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	st := readState()
	if st.Status == "running" && st.UnitName != "" && !unitActive(st.UnitName) {
		// Estado dizia "rodando" mas a unit transiente não existe mais viva
		// (host reiniciou no meio, ou o processo morreu sem escrever o
		// estado final) — autocura pra "unknown" em vez de travar o botão
		// de atualizar pra sempre.
		st.Status = "unknown"
		writeState(st)
	}
	st.LogTail = redact(tailFile(logPath, maxLogLines))
	writeJSON(w, st)
}

func handleApply(w http.ResponseWriter, r *http.Request) {
	current := readState()
	if current.Status == "running" && current.UnitName != "" && unitActive(current.UnitName) {
		writeError(w, http.StatusConflict, "atualização já em andamento")
		return
	}

	repoDir := os.Getenv("GESTPG_REPO_DIR")
	unitName := fmt.Sprintf("gestpg-update-run-%d.service", time.Now().Unix())

	// Trunca o log da execução anterior — só a última corrida importa,
	// sem histórico multi-run (evita crescer sem limite, sem valor extra
	// pra manter execuções antigas).
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("falha preparando log: %v", err))
		return
	}

	st := State{Status: "running", StartedAt: time.Now().Format(time.RFC3339), UnitName: unitName}
	if err := writeStateErr(st); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("falha gravando estado: %v", err))
		return
	}

	cmd := exec.Command("systemd-run",
		"--unit="+unitName,
		"--collect",
		"--quiet",
		"--setenv=REPO_DIR="+repoDir,
		"--setenv=LOG_PATH="+logPath,
		"--setenv=STATE_PATH="+statePath,
		"--setenv=UNIT_NAME="+unitName,
		"--setenv=HOME=/root",
		"/bin/sh", "-c", "{ "+updateScript+"} > \"$LOG_PATH\" 2>&1",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		ec := -1
		writeState(State{Status: "failed", StartedAt: st.StartedAt, FinishedAt: time.Now().Format(time.RFC3339),
			ExitCode: &ec, Error: fmt.Sprintf("falha disparando systemd-run: %v: %s", err, out)})
		writeError(w, http.StatusInternalServerError, "falha disparando atualização — checa se systemd-run está disponível no host")
		return
	}

	writeJSON(w, map[string]string{"status": "started", "unit_name": unitName})
}

func unitActive(unit string) bool {
	return exec.Command("systemctl", "is-active", "--quiet", unit).Run() == nil
}

func readState() State {
	data, err := os.ReadFile(statePath)
	if err != nil {
		return State{Status: "idle"}
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{Status: "idle"}
	}
	return st
}

func writeState(st State) {
	_ = writeStateErr(st)
}

func writeStateErr(st State) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0o600)
}

func tailFile(path string, maxLines int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}

// redactLoginLine esconde a senha de admin que `setup.sh` ecoa no fim de
// TODA execução bem-sucedida (não só na primeira instalação) — sem isso o
// log dessa atualização devolveria a senha em texto puro pra quem tiver
// sessão admin na API, aumentando à toa quem consegue ver o segredo.
var redactLoginLine = regexp.MustCompile(`(?m)^(\s*login:\s*admin\s*/\s*).+$`)

// ansiCodeRegex tira os códigos de cor (\033[1;34m etc) que log()/ok()/
// warn()/die() do setup.sh imprimem — sem isso o log renderizado no
// navegador vira sopa de caracteres de escape em vez de texto legível.
var ansiCodeRegex = regexp.MustCompile("\x1b\\[[0-9;]*m")

func redact(s string) string {
	s = ansiCodeRegex.ReplaceAllString(s, "")
	return redactLoginLine.ReplaceAllString(s, "${1}[redacted]")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
