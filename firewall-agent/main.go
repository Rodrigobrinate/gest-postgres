// Comando firewall-agent roda DIRETO no host (nunca em container — ufw
// manipula o namespace de rede do host, não tem como isso funcionar de
// dentro de um container sem dar privilégio que quebraria o modelo de
// segurança do resto da plataforma). Escuta só num socket Unix local,
// nunca porta de rede — o backend fala com ele via um bind mount desse
// socket, mesmo espírito do docker-socket-proxy: um mediador estreito, não
// acesso total exposto.
//
// Superfície proposital MÍNIMA: listar regras, liberar porta/protocolo
// (com origem opcional — IP/CIDR, ou "qualquer lugar" se vazio), remover
// regra por porta/protocolo/origem. `ufw enable`/`disable`/`--force reset`
// NUNCA são operações expostas por essa API — não existe endpoint pra isso,
// não é uma questão de validação que dá pra contornar. E a porta 22/tcp
// (SSH) nunca pode ser alterada por aqui em hipótese nenhuma, também
// travado em código, não em confirmação de UI — mesmo com origem
// restringida, esse caminho continua bloqueado de propósito (simplicidade
// > flexibilidade nesse ponto específico).
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
	"strconv"
	"strings"
)

const socketPath = "/run/gestpg-firewall.sock"

type Rule struct {
	Port   int    `json:"port"`
	Proto  string `json:"proto"`
	Action string `json:"action"`
	// From: IP ou CIDR de origem — vazio = qualquer lugar (comportamento de
	// sempre). "203.0.113.5" ou "203.0.113.0/24".
	From string `json:"from,omitempty"`
}

func main() {
	os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("escutando socket %s: %v", socketPath, err)
	}
	// 0666: o backend roda dentro de um container com UID diferente do
	// processo desse agente no host, não dá pra alinhar UID/GID de forma
	// simples — o socket só é alcançável por quem já tem acesso ao
	// filesystem do host montado no container (bind mount explícito no
	// docker-compose.yml), então isso não abre nada que já não estivesse
	// implicitamente acessível por quem controla o backend.
	if err := os.Chmod(socketPath, 0o666); err != nil {
		log.Fatalf("ajustando permissão do socket: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /rules", handleList)
	mux.HandleFunc("POST /rules", handleAdd)
	mux.HandleFunc("DELETE /rules/{port}/{proto}", handleDelete)

	log.Printf("firewall-agent escutando em %s", socketPath)
	log.Fatal(http.Serve(listener, mux))
}

// ruleLineRegex casa a linha de `ufw status` — grupo 6 é o "From" cru
// ("Anywhere", "Anywhere (v6)" ou um IP/CIDR de verdade).
var ruleLineRegex = regexp.MustCompile(`^(\d+)(/(tcp|udp))?\s*(\(v6\))?\s+(ALLOW|DENY|REJECT)(?:\s+IN)?\s+(\S+)`)

// handleList mostra o estado de verdade quando o ufw tá ativo (`ufw
// status`), mas cai pra `ufw show added` quando ainda tá inativo — sem
// isso, uma regra adicionada ANTES do primeiro `ufw enable` simplesmente
// some da listagem (`ufw status` não mostra nada com o firewall desligado,
// mesmo com regras já gravadas), o que confundiria muito no primeiro uso.
func handleList(w http.ResponseWriter, r *http.Request) {
	statusOut, err := exec.Command("ufw", "status").CombinedOutput()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("ufw status falhou: %v: %s", err, statusOut))
		return
	}

	var rules []Rule
	if strings.Contains(string(statusOut), "Status: active") {
		rules = parseStatusRules(string(statusOut))
	} else {
		addedOut, err := exec.Command("ufw", "show", "added").CombinedOutput()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("ufw show added falhou: %v: %s", err, addedOut))
			return
		}
		rules = parseAddedRules(string(addedOut))
	}
	writeJSON(w, rules)
}

func normalizeFrom(raw string) string {
	raw = strings.TrimSuffix(raw, " (v6)")
	if raw == "Anywhere" || raw == "Anywhere (v6)" || raw == "" {
		return ""
	}
	return raw
}

func parseStatusRules(status string) []Rule {
	seen := map[string]bool{}
	var rules []Rule
	for _, line := range strings.Split(status, "\n") {
		m := ruleLineRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		port, _ := strconv.Atoi(m[1])
		proto := m[3]
		if proto == "" {
			proto = "tcp"
		}
		action := strings.ToLower(m[5])
		from := normalizeFrom(m[6])
		key := fmt.Sprintf("%d/%s/%s/%s", port, proto, action, from)
		if seen[key] {
			continue // v4 e v6 da mesma regra viram uma entrada só
		}
		seen[key] = true
		rules = append(rules, Rule{Port: port, Proto: proto, Action: action, From: from})
	}
	return rules
}

var addedLineRegex = regexp.MustCompile(`^ufw (allow|deny) (\d+)(/(tcp|udp))?\s*$`)
var addedFromLineRegex = regexp.MustCompile(`^ufw (allow|deny) from (\S+) to any port (\d+)(?: proto (tcp|udp))?`)

func parseAddedRules(added string) []Rule {
	seen := map[string]bool{}
	var rules []Rule
	for _, line := range strings.Split(added, "\n") {
		line = strings.TrimSpace(line)
		if m := addedFromLineRegex.FindStringSubmatch(line); m != nil {
			action := m[1]
			from := m[2]
			port, _ := strconv.Atoi(m[3])
			proto := m[4]
			if proto == "" {
				proto = "tcp"
			}
			key := fmt.Sprintf("%d/%s/%s/%s", port, proto, action, from)
			if seen[key] {
				continue
			}
			seen[key] = true
			rules = append(rules, Rule{Port: port, Proto: proto, Action: action, From: from})
			continue
		}
		m := addedLineRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		action := m[1]
		port, _ := strconv.Atoi(m[2])
		proto := m[4]
		if proto == "" {
			proto = "tcp"
		}
		key := fmt.Sprintf("%d/%s/%s/", port, proto, action)
		if seen[key] {
			continue
		}
		seen[key] = true
		rules = append(rules, Rule{Port: port, Proto: proto, Action: action})
	}
	return rules
}

func ufwArgs(action string, r Rule) []string {
	if r.From == "" {
		return []string{action, fmt.Sprintf("%d/%s", r.Port, r.Proto)}
	}
	return []string{action, "from", r.From, "to", "any", "port", strconv.Itoa(r.Port), "proto", r.Proto}
}

func handleAdd(w http.ResponseWriter, r *http.Request) {
	var in Rule
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if err := validateRule(in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	action := "allow"
	if in.Action == "deny" {
		action = "deny"
	}
	out, err := exec.Command("ufw", ufwArgs(action, in)...).CombinedOutput()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("ufw %s falhou: %v: %s", action, err, out))
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	port, convErr := strconv.Atoi(r.PathValue("port"))
	proto := r.PathValue("proto")
	from := r.URL.Query().Get("from")
	if convErr != nil {
		writeError(w, http.StatusBadRequest, "porta inválida")
		return
	}
	rule := Rule{Port: port, Proto: proto, Action: "allow", From: from}
	if err := validateRule(rule); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Tenta os dois sentidos (a regra pode ter sido allow ou deny) — `ufw
	// delete` numa regra que não existe só devolve um erro inofensivo tipo
	// "Could not delete non-existent rule", ignorado de propósito.
	exec.Command("ufw", append([]string{"delete"}, ufwArgs("allow", rule)...)...).Run()
	exec.Command("ufw", append([]string{"delete"}, ufwArgs("deny", rule)...)...).Run()
	writeJSON(w, map[string]string{"status": "ok"})
}

// validateRule é a trava dura: porta 22/tcp (SSH) nunca pode ser alterada
// por essa API, nem allow nem deny, com ou sem origem restrita — não é uma
// confirmação de UI que dá pra pular, é o código recusando o pedido antes
// de chegar perto do `ufw`.
func validateRule(r Rule) error {
	if r.Proto != "tcp" && r.Proto != "udp" {
		return fmt.Errorf("protocolo deve ser tcp ou udp")
	}
	if r.Port <= 0 || r.Port > 65535 {
		return fmt.Errorf("porta inválida")
	}
	if r.Port == 22 && r.Proto == "tcp" {
		return fmt.Errorf("porta 22/tcp (SSH) nunca pode ser alterada por aqui — protege contra perder acesso remoto ao servidor")
	}
	if r.From != "" {
		if net.ParseIP(r.From) == nil {
			if _, _, err := net.ParseCIDR(r.From); err != nil {
				return fmt.Errorf("origem inválida — use um IP (ex: 203.0.113.5) ou CIDR (ex: 203.0.113.0/24)")
			}
		}
	}
	return nil
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
