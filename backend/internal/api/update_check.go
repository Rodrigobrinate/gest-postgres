package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/version"
)

const updateCheckRepo = "Rodrigobrinate/gest-postgres"

type UpdateCheckResult struct {
	CurrentCommit string `json:"current_commit"`
	// Unknown: o binário rodando foi buildado sem GIT_COMMIT (ex: build
	// manual fora do setup.sh) — não dá pra comparar com nada.
	Unknown          bool   `json:"unknown"`
	LatestCommit     string `json:"latest_commit,omitempty"`
	LatestCommitDate string `json:"latest_commit_date,omitempty"`
	LatestMessage    string `json:"latest_commit_message,omitempty"`
	UpToDate         bool   `json:"up_to_date"`
	CompareURL       string `json:"compare_url,omitempty"`
}

type githubCommitResponse struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

// CheckUpdate consulta o HEAD real do branch main no GitHub (API pública,
// sem token — repo público, e isso é só uma leitura de metadado de commit,
// não justifica gerenciar credencial) e compara com o commit embutido no
// binário em build time (ver internal/version). Nunca aplica nada sozinho —
// só informa; atualizar continua sendo `git pull && sudo ./setup.sh` rodado
// por quem administra o host. Função solta (não método de handler) — não
// depende de nenhum service, é só um proxy pro GitHub + comparação.
func CheckUpdate(w http.ResponseWriter, r *http.Request) {
	result := UpdateCheckResult{CurrentCommit: version.Commit}
	if version.Commit == "dev" {
		result.Unknown = true
		httpx.WriteJSON(w, http.StatusOK, result)
		return
	}

	reqCtx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet,
		"https://api.github.com/repos/"+updateCheckRepo+"/commits/main", nil)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "falha montando checagem de atualização")
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, fmt.Sprintf("não consegui alcançar o GitHub pra checar atualização: %v", err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// 403 com X-RateLimit-Remaining: 0 é o caso mais comum de "conexão
		// ok mas checagem falha" — a API pública do GitHub sem token limita
		// 60 requisições/hora POR IP (não por instalação), então um droplet
		// que já gastou a cota (várias instalações atrás do mesmo IP, ou
		// só cliques repetidos em "verificar de novo") passa a falhar até a
		// janela resetar, mesmo com internet perfeita.
		msg := fmt.Sprintf("GitHub respondeu status %d checando atualização", resp.StatusCode)
		if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
			reset := resp.Header.Get("X-RateLimit-Reset")
			msg = "limite de requisições da API pública do GitHub esgotado pra esse IP (60/hora, sem token) — tenta de novo mais tarde"
			if reset != "" {
				if unixSec, err := strconv.ParseInt(reset, 10, 64); err == nil {
					msg += fmt.Sprintf(" (reseta às %s UTC)", time.Unix(unixSec, 0).UTC().Format("15:04"))
				}
			}
		}
		httpx.WriteError(w, http.StatusBadGateway, msg)
		return
	}

	var gh githubCommitResponse
	if err := json.NewDecoder(resp.Body).Decode(&gh); err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "resposta do GitHub em formato inesperado")
		return
	}

	result.LatestCommit = gh.SHA
	result.LatestCommitDate = gh.Commit.Author.Date
	if idx := strings.IndexByte(gh.Commit.Message, '\n'); idx >= 0 {
		result.LatestMessage = gh.Commit.Message[:idx]
	} else {
		result.LatestMessage = gh.Commit.Message
	}
	result.UpToDate = strings.HasPrefix(gh.SHA, version.Commit)
	result.CompareURL = "https://github.com/" + updateCheckRepo + "/compare/" + version.Commit + "...main"

	httpx.WriteJSON(w, http.StatusOK, result)
}

// update-agent (processo separado, roda no HOST via systemd — ver setup.sh
// e update-agent/) escuta nesse socket. O backend fala com ele por bind
// mount (docker-compose.yml), mesmo padrão do firewall-agent: um mediador
// estreito no meio, backend nunca ganha acesso de host direto. A pipeline
// que o agente dispara é fixa (git pull + ./setup.sh do repo configurado em
// install time) — essa API não manda comando nenhum pro agente, só
// aciona/consulta.
const updateAgentSocketPath = "/run/gestpg-update.sock"

var updateAgentClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", updateAgentSocketPath)
		},
	},
}

func updateAgentRequest(ctx context.Context, method, path string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://unix"+path, bytes.NewReader(nil))
	if err != nil {
		return 0, err
	}
	resp, err := updateAgentClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("update-agent indisponível — precisa estar rodando no host (ver setup.sh): %w", err)
	}
	defer resp.Body.Close()

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

type ApplyUpdateResult struct {
	Status     string `json:"status"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	Error      string `json:"error,omitempty"`
	LogTail    string `json:"log_tail,omitempty"`
}

// UpdateStatus reporta o estado da última atualização disparada (idle se
// nunca rodou). Admin-only — o log de execução pode conter detalhe de
// build/deploy que um viewer não precisa enxergar (a senha de admin que o
// próprio setup.sh ecoa no fim já é redigida pelo update-agent antes de
// chegar aqui, mas o resto do log ainda é informação operacional restrita).
func UpdateStatus(w http.ResponseWriter, r *http.Request) {
	var result ApplyUpdateResult
	status, err := updateAgentRequest(r.Context(), http.MethodGet, "/status", &result)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}
	if status >= 300 {
		httpx.WriteError(w, http.StatusBadGateway, "update-agent respondeu status inesperado")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

// ApplyUpdate dispara `git pull && ./setup.sh` no host, via update-agent.
// Não aceita corpo — a pipeline é fixa do lado do agente. Exige sessão
// admin ELEVADA (step-up de senha, ver requireElevated) — é a ação de
// maior blast radius de toda a API (roda comando com privilégio de root no
// host inteiro, não só num container), então recebe a trava mais forte que
// a plataforma já tem, mesmo padrão do file manager do host.
func ApplyUpdate(w http.ResponseWriter, r *http.Request) {
	var result map[string]string
	status, err := updateAgentRequest(r.Context(), http.MethodPost, "/apply", &result)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}
	if status == http.StatusConflict {
		httpx.WriteError(w, http.StatusConflict, "atualização já em andamento")
		return
	}
	if status >= 300 {
		httpx.WriteError(w, http.StatusBadGateway, "update-agent recusou disparar a atualização")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}
