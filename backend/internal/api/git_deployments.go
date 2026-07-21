package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/infra"
)

// webhookThrottle limita tentativa por IP de origem nessa rota — a ÚNICA da
// API que não passa por sessão de usuário (autenticada só pela assinatura do
// provedor), então não tem o throttle de login pra segurar tentativa
// repetida de adivinhar ID de deployment + segredo (achado de auditoria).
// Mesmo espírito do throttle de login (backoff exponencial por chave,
// reseta no sucesso), instância própria — deployment nem sempre existe
// ainda quando checamos, então não dá pra reaproveitar o throttle de login
// sem acoplar os dois pacotes por pouco benefício.
type webhookThrottle struct {
	mu       sync.Mutex
	attempts map[string]*webhookAttemptState
}

type webhookAttemptState struct {
	failures    int
	lockedUntil time.Time
}

var webhookLimiter = &webhookThrottle{attempts: make(map[string]*webhookAttemptState)}

func (t *webhookThrottle) locked(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	st, ok := t.attempts[key]
	return ok && time.Now().Before(st.lockedUntil)
}

func (t *webhookThrottle) recordFailure(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	st, ok := t.attempts[key]
	if !ok {
		st = &webhookAttemptState{}
		t.attempts[key] = st
	}
	st.failures++
	backoff := time.Duration(1<<uint(min(st.failures, 7))) * time.Second
	st.lockedUntil = time.Now().Add(backoff)
}

func (t *webhookThrottle) recordSuccess(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.attempts, key)
}

type GitDeploymentsHandler struct {
	service *infra.Service
}

func NewGitDeploymentsHandler(service *infra.Service) *GitDeploymentsHandler {
	return &GitDeploymentsHandler{service: service}
}

func (h *GitDeploymentsHandler) List(w http.ResponseWriter, r *http.Request) {
	deployments, err := h.service.ListGitDeployments(r.Context())
	if err != nil {
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, deployments)
}

func (h *GitDeploymentsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in infra.CreateGitDeploymentInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	result, err := h.service.CreateGitDeployment(r.Context(), in)
	if err != nil {
		if result != nil {
			httpx.WriteJSON(w, http.StatusUnprocessableEntity, result)
			return
		}
		writeInfraError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, result)
}

func (h *GitDeploymentsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteGitDeployment(r.Context(), r.PathValue("deploymentId")); err != nil {
		writeInfraError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *GitDeploymentsHandler) RedeployNow(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RedeployFromGit(r.Context(), r.PathValue("deploymentId")); err != nil {
		httpx.WriteError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Webhook é rota PÚBLICA (ver publicPathSuffixes em internal/api/middleware.go
// — sufixo "/webhook" pula withAuth) porque quem chama é o GitHub/GitLab, não
// um usuário logado. Quem autentica de verdade é a assinatura do provedor,
// conferida contra o segredo gerado na criação do deployment. Suporta os dois
// formatos mais comuns: GitHub (X-Hub-Signature-256, HMAC-SHA256 do corpo
// inteiro) e GitLab (X-Gitlab-Token, comparação direta de token).
func (h *GitDeploymentsHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if webhookLimiter.locked(ip) {
		httpx.WriteError(w, http.StatusTooManyRequests, "muitas tentativas — aguarde antes de tentar de novo")
		return
	}

	id := r.PathValue("deploymentId")
	secret, err := h.service.GitWebhookSecret(r.Context(), id)
	if err != nil {
		// Mesma resposta de "assinatura inválida" — id inexistente e
		// assinatura errada precisam ser indistinguíveis pra fora, senão o
		// status code vira oráculo pra enumerar deployment válido (achado
		// de auditoria; mitigado pelo id já ser UUID, mas sem motivo pra
		// manter a distinção).
		webhookLimiter.recordFailure(ip)
		httpx.WriteError(w, http.StatusUnauthorized, "assinatura inválida")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	if !verifyGitWebhookAuth(r, body, secret) {
		webhookLimiter.recordFailure(ip)
		httpx.WriteError(w, http.StatusUnauthorized, "assinatura inválida")
		return
	}
	webhookLimiter.recordSuccess(ip)

	// Redeploy roda em background — GitHub/GitLab esperam resposta rápida
	// do webhook (timeout curto), e clone+build pode demorar bem mais que
	// isso.
	go func() {
		_ = h.service.RedeployFromGit(context.Background(), id)
	}()

	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "deploy disparado"})
}

func verifyGitWebhookAuth(r *http.Request, body []byte, secret string) bool {
	if sig := r.Header.Get("X-Hub-Signature-256"); sig != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(sig), []byte(expected))
	}
	if token := r.Header.Get("X-Gitlab-Token"); token != "" {
		return subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1
	}
	return false
}
