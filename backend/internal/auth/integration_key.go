package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// integrationKeyPrefix vai colado no início da chave em texto puro —
// grepável em log (mesmo raciocínio de secret prefixado tipo Stripe),
// nunca usado na validação em si (só o hash da string inteira importa).
const integrationKeyPrefix = "gpgik_"

// IntegrationKeyStatus é o que a UI local mostra sobre a chave ativa — nunca
// o segredo em si, só metadado (mesmo padrão de nunca reexibir senha salva
// em nenhum outro lugar do projeto).
type IntegrationKeyStatus struct {
	Active     bool       `json:"active"`
	Label      string     `json:"label,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

func newIntegrationKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return integrationKeyPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

// RotateIntegrationKey revoga a chave ativa (se existir) e cria uma nova, na
// mesma transação — o índice único parcial em integration_keys garante que
// nunca sobra mais de uma ativa ao mesmo tempo (ver migration 0021). Chave
// em texto puro só existe nesse retorno, nunca mais reexibida.
func (s *Service) RotateIntegrationKey(ctx context.Context, label string) (string, error) {
	key, err := newIntegrationKey()
	if err != nil {
		return "", fmt.Errorf("gerando chave: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("iniciando transação: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE integration_keys SET revoked_at = now() WHERE revoked_at IS NULL`); err != nil {
		return "", fmt.Errorf("revogando chave anterior: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO integration_keys (key_hash, label) VALUES ($1, $2)
	`, hashToken(key), label); err != nil {
		return "", fmt.Errorf("salvando chave nova: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("confirmando rotação: %w", err)
	}
	return key, nil
}

// RevokeIntegrationKey desativa a chave ativa sem gerar outra — modo
// "desconectar do mestre" sem reconectar na hora.
func (s *Service) RevokeIntegrationKey(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `UPDATE integration_keys SET revoked_at = now() WHERE revoked_at IS NULL`); err != nil {
		return fmt.Errorf("revogando chave: %w", err)
	}
	return nil
}

func (s *Service) IntegrationKeyStatus(ctx context.Context) (*IntegrationKeyStatus, error) {
	var st IntegrationKeyStatus
	var createdAt time.Time
	var lastUsedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT label, created_at, last_used_at FROM integration_keys WHERE revoked_at IS NULL
	`).Scan(&st.Label, &createdAt, &lastUsedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &IntegrationKeyStatus{Active: false}, nil
		}
		return nil, fmt.Errorf("lendo chave de integração: %w", err)
	}
	st.Active = true
	st.CreatedAt = &createdAt
	st.LastUsedAt = lastUsedAt
	return &st, nil
}

// ValidateIntegrationKey confere a chave e marca uso — single query, mesmo
// espírito de ValidateSession (valida e estende/atualiza numa tacada só).
func (s *Service) ValidateIntegrationKey(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, nil
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE integration_keys SET last_used_at = now()
		WHERE key_hash = $1 AND revoked_at IS NULL
	`, hashToken(key))
	if err != nil {
		return false, fmt.Errorf("validando chave de integração: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// SeedIntegrationKeyIfProvided grava no boot a chave que o painel do mestre
// gerou (setup.sh --integration-key) — diferente de RotateIntegrationKey
// (que GERA uma chave nova), aqui o valor já vem pronto de fora, o backend
// só precisa guardar o hash dele. Idempotente entre reinícios: se a chave
// ativa já tem esse mesmo hash, não faz nada (evita acumular histórico de
// revogação a cada restart do backend enquanto a env var continuar igual).
func (s *Service) SeedIntegrationKeyIfProvided(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	th := hashToken(key)

	var existing string
	err := s.pool.QueryRow(ctx, `SELECT key_hash FROM integration_keys WHERE revoked_at IS NULL`).Scan(&existing)
	if err == nil && existing == th {
		return nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("lendo chave de integração ativa: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciando transação: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE integration_keys SET revoked_at = now() WHERE revoked_at IS NULL`); err != nil {
		return fmt.Errorf("revogando chave anterior: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO integration_keys (key_hash, label) VALUES ($1, 'seed via setup.sh')
	`, th); err != nil {
		return fmt.Errorf("salvando chave semeada: %w", err)
	}
	return tx.Commit(ctx)
}

// NewServiceSession representa uma requisição autenticada pela chave de
// integração (o Worker do futuro sistema mestre) — sem expiração em banco
// (a chave em si já controla isso via revoked_at), papel sempre admin: "tudo
// que o frontend local faz, o mestre também faz" foi decisão explícita do
// usuário no planejamento desse recurso.
func NewServiceSession() *Session {
	return &Session{
		ID:       "integration-key",
		Username: "master-control-plane",
		Role:     RoleAdmin,
	}
}
