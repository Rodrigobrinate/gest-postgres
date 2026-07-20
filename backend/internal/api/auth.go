package api

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gest-postgres/backend/internal/auth"
	"github.com/gest-postgres/backend/internal/httpx"
)

type AuthHandler struct {
	service *auth.Service
}

func NewAuthHandler(service *auth.Service) *AuthHandler {
	return &AuthHandler{service: service}
}

// isRequestHTTPS detecta se a requisição chegou por TLS — direto (r.TLS,
// raro já que a instalação padrão publica a porta do backend crua) ou via
// X-Forwarded-Proto (quando tem Traefik/outro proxy terminando TLS na
// frente, mesmo sinal que redirectURL() já usa pro callback do OAuth).
func isRequestHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// setSessionCookie marca Secure quando dá pra confirmar que a conexão é
// HTTPS — nunca hardcoded, porque a instalação padrão (sem domínio/Traefik
// na frente ainda) serve a API em HTTP puro, e Secure:true quebraria login
// nesse caso (navegador nunca reenvia o cookie numa conexão não-criptografada).
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isRequestHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// clientIP extrai o IP de origem pro throttle de login — X-Forwarded-For
// primeiro (Traefik/outro proxy na frente já pode estar em uso, ver infra
// Traefik) senão RemoteAddr direto.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := strings.IndexByte(fwd, ','); i >= 0 {
			return strings.TrimSpace(fwd[:i])
		}
		return strings.TrimSpace(fwd)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	token, err := h.service.Login(r.Context(), req.Username, req.Password, clientIP(r))
	if err != nil {
		if errors.Is(err, auth.ErrRateLimited) {
			httpx.WriteError(w, http.StatusTooManyRequests, "muitas tentativas — aguarde antes de tentar de novo")
			return
		}
		if errors.Is(err, auth.ErrInvalidCredentials) {
			httpx.WriteError(w, http.StatusUnauthorized, "usuário ou senha inválidos")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	setSessionCookie(w, r, token, 30*24*60*60)
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_ = h.service.Logout(r.Context(), cookie.Value)
	}
	setSessionCookie(w, r, "", -1)
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	sess, ok := auth.SessionFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"username":      sess.Username,
		"role":          sess.Role,
	})
}

type stepUpRequest struct {
	Password string `json:"password"`
}

func (h *AuthHandler) StepUp(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	var req stepUpRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	elevatedUntil, err := h.service.StepUp(r.Context(), cookie.Value, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			httpx.WriteError(w, http.StatusUnauthorized, "senha incorreta")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]time.Time{"elevated_until": elevatedUntil})
}
