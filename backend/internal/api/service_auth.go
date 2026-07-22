package api

import (
	"context"
	"sync"
	"time"
)

// serviceAuthCtxKey marca uma requisição autenticada pela chave de
// integração (Authorization: Bearer, ver withAuth em middleware.go) em vez
// de cookie de sessão — checado por requireElevated (não existe "elevar"
// uma credencial de máquina, ela já é full-trust) e por clientIP (âncora de
// confiança vira posse da chave, não posição de rede).
type serviceAuthCtxKey struct{}

func withServiceAuth(ctx context.Context) context.Context {
	return context.WithValue(ctx, serviceAuthCtxKey{}, true)
}

func isIntegrationAuthed(ctx context.Context) bool {
	v, _ := ctx.Value(serviceAuthCtxKey{}).(bool)
	return v
}

// serviceKeyThrottle limita tentativa de adivinhar a chave de integração por
// IP de origem — cópia do padrão de webhookThrottle (git_deployments.go):
// essa rota também não tem sessão de usuário pra apoiar o throttle de login.
type serviceKeyThrottleT struct {
	mu       sync.Mutex
	attempts map[string]*serviceKeyAttemptState
}

type serviceKeyAttemptState struct {
	failures    int
	lockedUntil time.Time
}

var serviceKeyThrottle = &serviceKeyThrottleT{attempts: make(map[string]*serviceKeyAttemptState)}

func (t *serviceKeyThrottleT) locked(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	st, ok := t.attempts[key]
	return ok && time.Now().Before(st.lockedUntil)
}

func (t *serviceKeyThrottleT) recordFailure(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	st, ok := t.attempts[key]
	if !ok {
		st = &serviceKeyAttemptState{}
		t.attempts[key] = st
	}
	st.failures++
	backoff := time.Duration(1<<uint(min(st.failures, 7))) * time.Second
	st.lockedUntil = time.Now().Add(backoff)
}

func (t *serviceKeyThrottleT) recordSuccess(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.attempts, key)
}
