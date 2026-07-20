package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("usuário ou senha inválidos")

const (
	sessionTTL = 30 * 24 * time.Hour
	stepUpTTL  = 5 * time.Minute
)

type Session struct {
	ID            string
	ExpiresAt     time.Time
	ElevatedUntil *time.Time
}

// Elevated diz se essa sessão passou por reconfirmação de senha (step-up)
// recente — usado só pelas rotas de escrita/exclusão do file manager do
// host, nunca globalmente (ver requireElevated em internal/api/middleware.go).
func (s *Session) Elevated() bool {
	return s.ElevatedUntil != nil && time.Now().Before(*s.ElevatedUntil)
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Login confere usuário/senha contra o admin_user singleton e cria uma
// sessão nova. Retorna o token em texto puro — só existe nesse retorno, o
// banco guarda somente o hash (mesmo raciocínio de nunca persistir segredo
// em texto puro usado no resto do projeto, ver internal/crypto).
func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	var storedUsername, hash string
	err := s.pool.QueryRow(ctx, `SELECT username, password_hash FROM admin_user WHERE id = 1`).Scan(&storedUsername, &hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrInvalidCredentials
		}
		return "", fmt.Errorf("lendo admin: %w", err)
	}

	if subtle.ConstantTimeCompare([]byte(username), []byte(storedUsername)) != 1 {
		return "", ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", ErrInvalidCredentials
	}

	token, err := newToken()
	if err != nil {
		return "", fmt.Errorf("gerando token de sessão: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO admin_sessions (token_hash, expires_at)
		VALUES ($1, $2)
	`, hashToken(token), time.Now().Add(sessionTTL))
	if err != nil {
		return "", fmt.Errorf("criando sessão: %w", err)
	}
	return token, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE token_hash = $1`, hashToken(token)); err != nil {
		return fmt.Errorf("removendo sessão: %w", err)
	}
	return nil
}

// ValidateSession confere o token e estende a expiração (janela deslizante)
// — uso contínuo da plataforma nunca expira no meio, só inatividade real.
func (s *Service) ValidateSession(ctx context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, ErrInvalidCredentials
	}
	th := hashToken(token)

	var sess Session
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, expires_at, elevated_until
		FROM admin_sessions
		WHERE token_hash = $1 AND expires_at > now()
	`, th).Scan(&sess.ID, &sess.ExpiresAt, &sess.ElevatedUntil)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("lendo sessão: %w", err)
	}

	newExpiry := time.Now().Add(sessionTTL)
	if _, err := s.pool.Exec(ctx, `
		UPDATE admin_sessions SET last_seen_at = now(), expires_at = $2 WHERE token_hash = $1
	`, th, newExpiry); err != nil {
		return nil, fmt.Errorf("estendendo sessão: %w", err)
	}
	sess.ExpiresAt = newExpiry

	return &sess, nil
}

// StepUp reconfirma a senha do admin e eleva a sessão atual por uma janela
// curta — pré-requisito pras rotas de escrita/exclusão do file manager do
// host (ver internal/infra/host_files.go). Não existe token elevado
// separado de propósito: mais simples, elevated_until vive na mesma sessão.
func (s *Service) StepUp(ctx context.Context, token, password string) (time.Time, error) {
	var hash string
	if err := s.pool.QueryRow(ctx, `SELECT password_hash FROM admin_user WHERE id = 1`).Scan(&hash); err != nil {
		return time.Time{}, fmt.Errorf("lendo admin: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return time.Time{}, ErrInvalidCredentials
	}

	elevatedUntil := time.Now().Add(stepUpTTL)
	tag, err := s.pool.Exec(ctx, `
		UPDATE admin_sessions SET elevated_until = $2 WHERE token_hash = $1 AND expires_at > now()
	`, hashToken(token), elevatedUntil)
	if err != nil {
		return time.Time{}, fmt.Errorf("elevando sessão: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return time.Time{}, ErrInvalidCredentials
	}
	return elevatedUntil, nil
}
