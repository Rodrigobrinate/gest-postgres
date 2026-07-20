package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

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

func postJSON(ctx context.Context, url string, body []byte) error {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("destino respondeu status %d", resp.StatusCode)
	}
	return nil
}
