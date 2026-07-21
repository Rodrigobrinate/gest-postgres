package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// cgnatBlock é o bloco de CGNAT (RFC 6598, 100.64.0.0/10) — não coberto por
// nenhum dos IP.Is* do stdlib (não é loopback/link-local/privado RFC1918),
// mas é roteável só dentro da rede do provedor de nuvem, mesmo espírito de
// link-local pra fins de SSRF (achado de auditoria: faltava no blocklist).
var cgnatBlock = func() *net.IPNet {
	_, n, err := net.ParseCIDR("100.64.0.0/10")
	if err != nil {
		panic(err)
	}
	return n
}()

func isBlockedSSRFIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsPrivate() || cgnatBlock.Contains(ip)
}

// validateWebhookURL fecha SSRF cego: sem isso, um admin (ou uma sessão
// abusada) cadastra um alerta/canal apontando pra serviço interno alcançável
// do backend — metadata de nuvem (169.254.169.254), o próprio
// docker-socket-proxy, localhost. Resolve o host e rejeita qualquer IP
// loopback/link-local/privado/CGNAT — checagem na CRIAÇÃO do canal/regra.
// DNS rebinding entre criação e disparo é mitigado à parte, ver
// safeDialContext (o dialer do cliente que efetivamente envia o POST pina
// no IP validado, não reresolve o host na hora de conectar).
func validateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("%w: webhook_url inválida — precisa ser http:// ou https://", ErrValidation)
	}
	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		return fmt.Errorf("%w: não foi possível resolver o host de webhook_url", ErrValidation)
	}
	for _, ip := range ips {
		if isBlockedSSRFIP(ip) {
			return fmt.Errorf("%w: webhook_url não pode apontar pra endereço interno/privado", ErrValidation)
		}
	}
	return nil
}

type NotificationChannelKind string

const (
	ChannelTelegram NotificationChannelKind = "telegram"
	ChannelWebhook  NotificationChannelKind = "webhook"
)

// NotificationChannel é reutilizável entre várias regras de alerta — em vez
// de colar a mesma URL/token do Telegram em cada regra, cadastra o canal
// uma vez aqui e a regra só referencia o ID. Plataforma inteira, não por
// servidor (o bot do Telegram é um só por instalação, faz mais sentido
// assim do que duplicar por servidor).
type NotificationChannel struct {
	ID             string                  `json:"id"`
	Name           string                  `json:"name"`
	Kind           NotificationChannelKind `json:"kind"`
	WebhookURL     string                  `json:"webhook_url,omitempty"`
	TelegramChatID string                  `json:"telegram_chat_id,omitempty"`
	CreatedAt      time.Time               `json:"created_at"`
}

type CreateNotificationChannelInput struct {
	Name             string                  `json:"name"`
	Kind             NotificationChannelKind `json:"kind"`
	WebhookURL       string                  `json:"webhook_url,omitempty"`
	TelegramBotToken string                  `json:"telegram_bot_token,omitempty"`
	TelegramChatID   string                  `json:"telegram_chat_id,omitempty"`
}

func (s *Service) ListNotificationChannels(ctx context.Context) ([]NotificationChannel, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT id, name, kind, webhook_url, telegram_chat_id, created_at
		FROM notification_channels ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("listando canais de notificação: %w", err)
	}
	defer rows.Close()

	out := []NotificationChannel{}
	for rows.Next() {
		var c NotificationChannel
		if err := rows.Scan(&c.ID, &c.Name, &c.Kind, &c.WebhookURL, &c.TelegramChatID, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("lendo canal de notificação: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) CreateNotificationChannel(ctx context.Context, in CreateNotificationChannelInput) (*NotificationChannel, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: nome é obrigatório", ErrValidation)
	}
	if in.Kind != ChannelTelegram && in.Kind != ChannelWebhook {
		return nil, fmt.Errorf("%w: kind deve ser 'telegram' ou 'webhook'", ErrValidation)
	}

	var tokenEncrypted string
	if in.Kind == ChannelTelegram {
		if in.TelegramBotToken == "" || in.TelegramChatID == "" {
			return nil, fmt.Errorf("%w: bot token e chat ID são obrigatórios pra canal Telegram", ErrValidation)
		}
		var err error
		tokenEncrypted, err = s.secretBox.Seal(in.TelegramBotToken)
		if err != nil {
			return nil, fmt.Errorf("cifrando token do bot: %w", err)
		}
	} else if in.WebhookURL == "" {
		return nil, fmt.Errorf("%w: webhook_url é obrigatório pra canal webhook", ErrValidation)
	} else if err := validateWebhookURL(in.WebhookURL); err != nil {
		return nil, err
	}

	var c NotificationChannel
	err := s.repo.pool.QueryRow(ctx, `
		INSERT INTO notification_channels (name, kind, webhook_url, telegram_bot_token_encrypted, telegram_chat_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, kind, webhook_url, telegram_chat_id, created_at
	`, in.Name, in.Kind, in.WebhookURL, tokenEncrypted, in.TelegramChatID).Scan(
		&c.ID, &c.Name, &c.Kind, &c.WebhookURL, &c.TelegramChatID, &c.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("salvando canal (nome já em uso?): %w", err)
	}
	return &c, nil
}

func (s *Service) DeleteNotificationChannel(ctx context.Context, id string) error {
	if _, err := s.repo.pool.Exec(ctx, `DELETE FROM notification_channels WHERE id = $1`, id); err != nil {
		return fmt.Errorf("excluindo canal: %w", err)
	}
	return nil
}

// TestNotificationChannel dispara uma mensagem de teste — usado pelo botão
// "Testar" na UI, pra confirmar token/chat ID/URL antes de depender disso
// num alerta de verdade.
func (s *Service) TestNotificationChannel(ctx context.Context, id string) error {
	channel, err := s.getNotificationChannel(ctx, id)
	if err != nil {
		return err
	}
	return s.sendNotification(ctx, channel, webhookPayload{
		Metric:      "teste",
		Description: "Mensagem de teste do gest-postgres",
		Value:       0,
		Threshold:   0,
		TriggeredAt: time.Now(),
	})
}

func (s *Service) getNotificationChannel(ctx context.Context, id string) (*NotificationChannel, error) {
	var c NotificationChannel
	var tokenEncrypted string
	err := s.repo.pool.QueryRow(ctx, `
		SELECT id, name, kind, webhook_url, telegram_bot_token_encrypted, telegram_chat_id, created_at
		FROM notification_channels WHERE id = $1
	`, id).Scan(&c.ID, &c.Name, &c.Kind, &c.WebhookURL, &tokenEncrypted, &c.TelegramChatID, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("lendo canal de notificação: %w", err)
	}
	if tokenEncrypted != "" {
		token, err := s.secretBox.Open(tokenEncrypted)
		if err != nil {
			return nil, fmt.Errorf("decifrando token do bot: %w", err)
		}
		c.WebhookURL = "https://api.telegram.org/bot" + token + "/sendMessage"
	}
	return &c, nil
}

// sendNotification manda pro canal certo — telegram vira uma mensagem de
// texto formatada (a API do bot só aceita isso, não JSON arbitrário como
// webhook genérico), webhook manda o payload cru de sempre.
func (s *Service) sendNotification(ctx context.Context, channel *NotificationChannel, payload webhookPayload) error {
	if channel.Kind == ChannelTelegram {
		text := fmt.Sprintf("🔔 *%s*\n%s\nValor: %.2f (limite: %.2f)\n%s",
			payload.ServerName, payload.Description, payload.Value, payload.Threshold, payload.TriggeredAt.Format("02/01 15:04"))
		body, _ := json.Marshal(map[string]string{
			"chat_id":    channel.TelegramChatID,
			"text":       text,
			"parse_mode": "Markdown",
		})
		return postJSON(ctx, channel.WebhookURL, body)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postJSON(ctx, channel.WebhookURL, body)
}

// botTokenInURLRegex casa o token do bot embutido na URL do Telegram
// (channel.WebhookURL = ".../bot<TOKEN>/sendMessage") — erro de transporte
// do http.Client (*url.Error) inclui a URL INTEIRA na mensagem, então sem
// redigir isso aqui o token vaza tanto no log de alerta quanto no corpo 422
// devolvido pelo botão "Testar" da UI.
var botTokenInURLRegex = regexp.MustCompile(`(api\.telegram\.org/bot)[^/]+`)

func redactSecretsInError(err error) error {
	if err == nil {
		return nil
	}
	redacted := botTokenInURLRegex.ReplaceAllString(err.Error(), "${1}<redacted>")
	return fmt.Errorf("%s", redacted)
}

// safeDialContext resolve o host e recusa conectar se QUALQUER IP resolvido
// for interno/privado/CGNAT, depois conecta direto no IP já validado — nunca
// deixa o transport HTTP padrão reresolver o hostname na hora de conectar.
// Fecha o TOCTOU de DNS rebinding entre validateWebhookURL (checagem na
// criação do canal) e o disparo de verdade: um host que respondia IP público
// no cadastro e passa a responder IP interno no disparo é pego aqui, não só
// na criação (achado de auditoria).
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("não foi possível resolver %q", host)
	}
	for _, ip := range ips {
		if isBlockedSSRFIP(ip.IP) {
			return nil, fmt.Errorf("destino resolve pra endereço interno/privado")
		}
	}
	var d net.Dialer
	return d.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

// noRedirectClient nunca segue redirect — um alvo malicioso poderia
// responder 30x apontando pra endereço interno depois da validação de
// SSRF já ter passado no host original. DialContext pinado (ver
// safeDialContext) fecha o rebind entre a validação na criação do canal e
// o disparo de verdade.
var noRedirectClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Transport: &http.Transport{
		DialContext: safeDialContext,
	},
}

func postJSON(ctx context.Context, targetURL string, body []byte) error {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return redactSecretsInError(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return redactSecretsInError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("destino respondeu status %d", resp.StatusCode)
	}
	return nil
}
