package api

import (
	"errors"
	"net/http"
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

func setSessionCookie(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	token, err := h.service.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			httpx.WriteError(w, http.StatusUnauthorized, "usuário ou senha inválidos")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	setSessionCookie(w, token, 30*24*60*60)
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_ = h.service.Logout(r.Context(), cookie.Value)
	}
	setSessionCookie(w, "", -1)
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
