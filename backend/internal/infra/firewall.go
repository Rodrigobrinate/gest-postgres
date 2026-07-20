package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// firewallSocketPath é onde o firewall-agent (processo separado, roda no
// HOST via systemd — ver setup.sh e firewall-agent/) escuta. O backend fala
// com ele por esse socket via bind mount (docker-compose.yml), nunca ganha
// acesso de rede/privilégio de host direto — mesmo padrão do
// docker-socket-proxy, um mediador estreito no meio.
const firewallSocketPath = "/run/gestpg-firewall.sock"

var firewallClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", firewallSocketPath)
		},
	},
}

type FirewallRule struct {
	Port   int    `json:"port"`
	Proto  string `json:"proto"`
	Action string `json:"action"`
	From   string `json:"from,omitempty"`
}

func firewallRequest(ctx context.Context, method, path string, body any, out any) error {
	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = &bytes.Buffer{}
	}

	req, err := http.NewRequestWithContext(ctx, method, "http://unix"+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := firewallClient.Do(req)
	if err != nil {
		return fmt.Errorf("firewall-agent indisponível — precisa estar rodando no host (ver setup.sh): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		if errBody.Error != "" {
			return fmt.Errorf("%s", errBody.Error)
		}
		return fmt.Errorf("firewall-agent: status %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (s *Service) ListFirewallRules(ctx context.Context) ([]FirewallRule, error) {
	var rules []FirewallRule
	if err := firewallRequest(ctx, http.MethodGet, "/rules", nil, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

type AddFirewallRuleInput struct {
	Port   int    `json:"port"`
	Proto  string `json:"proto"`
	Action string `json:"action"`
	From   string `json:"from,omitempty"`
}

func (s *Service) AddFirewallRule(ctx context.Context, in AddFirewallRuleInput) error {
	return firewallRequest(ctx, http.MethodPost, "/rules", in, nil)
}

func (s *Service) RemoveFirewallRule(ctx context.Context, port int, proto, from string) error {
	path := fmt.Sprintf("/rules/%d/%s", port, proto)
	if from != "" {
		path += "?from=" + url.QueryEscape(from)
	}
	return firewallRequest(ctx, http.MethodDelete, path, nil, nil)
}
