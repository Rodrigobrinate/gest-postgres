package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("usuário ou senha inválidos")
var ErrRateLimited = errors.New("muitas tentativas — aguarde antes de tentar de novo")

const (
	sessionTTL = 30 * 24 * time.Hour
	stepUpTTL  = 5 * time.Minute
)

// dummyPasswordHash é comparado quando o username não existe, pra gastar o
// MESMO tempo de bcrypt que uma tentativa contra um usuário real — sem
// isso, "usuário não existe" retorna quase instantaneamente (sem chamar
// bcrypt) enquanto "senha errada" leva o tempo do bcrypt, permitindo
// enumerar usuários válidos só pelo timing da resposta.
var dummyPasswordHash = func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("gestpg-dummy-timing-safe"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return h
}()

// loginThrottle é um limitador simples em memória por IP de origem — chave
// por IP (não por username) de propósito: travar por username deixaria
// qualquer atacante bloquear o admin de verdade só errando a senha de
// "admin" repetidamente. Backoff exponencial, reseta ao acertar. Em memória
// (reseta se o backend reiniciar) — condizente com o resto da plataforma,
// que já é single-process, sem necessidade de coordenar entre réplicas.
type loginThrottle struct {
	mu       sync.Mutex
	attempts map[string]*loginAttemptState
}

type loginAttemptState struct {
	failures    int
	lockedUntil time.Time
}

var throttle = &loginThrottle{attempts: make(map[string]*loginAttemptState)}

func (t *loginThrottle) checkLocked(key string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if st, ok := t.attempts[key]; ok && time.Now().Before(st.lockedUntil) {
		return ErrRateLimited
	}
	return nil
}

func (t *loginThrottle) recordFailure(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	st, ok := t.attempts[key]
	if !ok {
		st = &loginAttemptState{}
		t.attempts[key] = st
	}
	st.failures++
	// 2^failures segundos, capado em ~2min — poucas tentativas erradas mal
	// atrasam, tentativa persistente de brute-force fica cada vez mais lenta.
	backoff := time.Duration(1<<uint(min(st.failures, 7))) * time.Second
	st.lockedUntil = time.Now().Add(backoff)
}

func (t *loginThrottle) recordSuccess(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.attempts, key)
}

type Session struct {
	ID            string
	UserID        string
	Username      string
	Role          Role
	ExpiresAt     time.Time
	ElevatedUntil *time.Time
}

func (s *Session) IsAdmin() bool {
	return s.Role == RoleAdmin
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

// Login confere usuário/senha contra a tabela users e cria uma sessão nova.
// Retorna o token em texto puro — só existe nesse retorno, o banco guarda
// somente o hash (mesmo raciocínio de nunca persistir segredo em texto
// puro usado no resto do projeto, ver internal/crypto). clientKey identifica
// quem está tentando (IP de origem) pro throttle — nunca o username, ver
// loginThrottle.
func (s *Service) Login(ctx context.Context, username, password, clientKey string) (string, error) {
	if err := throttle.checkLocked(clientKey); err != nil {
		return "", err
	}

	var userID, hash string
	var role Role
	err := s.pool.QueryRow(ctx, `SELECT id::text, password_hash, role FROM users WHERE username = $1`, username).
		Scan(&userID, &hash, &role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Compara contra um hash dummy mesmo sem usuário — gasta o
			// mesmo tempo de bcrypt que uma tentativa com usuário real,
			// fecha a diferença de timing que permitiria enumerar usuários.
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
			throttle.recordFailure(clientKey)
			return "", ErrInvalidCredentials
		}
		return "", fmt.Errorf("lendo usuário: %w", err)
	}

	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		throttle.recordFailure(clientKey)
		return "", ErrInvalidCredentials
	}
	throttle.recordSuccess(clientKey)

	token, err := newToken()
	if err != nil {
		return "", fmt.Errorf("gerando token de sessão: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO admin_sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, hashToken(token), userID, time.Now().Add(sessionTTL))
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

// ValidateSession confere o token, estende a expiração (janela deslizante
// — uso contínuo da plataforma nunca expira no meio, só inatividade real)
// e carrega usuário/papel numa tacada só (join com users).
func (s *Service) ValidateSession(ctx context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, ErrInvalidCredentials
	}
	th := hashToken(token)

	var sess Session
	err := s.pool.QueryRow(ctx, `
		SELECT s.id::text, s.expires_at, s.elevated_until, u.id::text, u.username, u.role
		FROM admin_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()
	`, th).Scan(&sess.ID, &sess.ExpiresAt, &sess.ElevatedUntil, &sess.UserID, &sess.Username, &sess.Role)
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

// StepUp reconfirma a senha do usuário da sessão atual e eleva ela por uma
// janela curta — pré-requisito pras rotas de escrita/exclusão do file
// manager do host (ver internal/infra/host_files.go). Não existe token
// elevado separado de propósito: mais simples, elevated_until vive na
// mesma sessão.
func (s *Service) StepUp(ctx context.Context, token, password string) (time.Time, error) {
	th := hashToken(token)

	var hash string
	err := s.pool.QueryRow(ctx, `
		SELECT u.password_hash
		FROM admin_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()
	`, th).Scan(&hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, ErrInvalidCredentials
		}
		return time.Time{}, fmt.Errorf("lendo sessão: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return time.Time{}, ErrInvalidCredentials
	}

	elevatedUntil := time.Now().Add(stepUpTTL)
	if _, err := s.pool.Exec(ctx, `
		UPDATE admin_sessions SET elevated_until = $2 WHERE token_hash = $1 AND expires_at > now()
	`, th, elevatedUntil); err != nil {
		return time.Time{}, fmt.Errorf("elevando sessão: %w", err)
	}
	return elevatedUntil, nil
}
