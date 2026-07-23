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

// trustedProxyNets é setado uma vez, na montagem do router (ver
// router.go/SetTrustedProxies), a partir de config.TrustedProxies — vazio
// por padrão. Enquanto vazio, X-Forwarded-For/-Proto NUNCA são honrados
// (instalação padrão publica a porta do backend crua, sem proxy real na
// frente — confiar nesses headers incondicionalmente deixa qualquer
// requisição direta forjar IP de origem e burlar o throttle de login, ou
// forjar "https" pra suprimir o cookie Secure). Só populado quando o admin
// configura TRUSTED_PROXIES porque tem um reverse proxy de verdade
// (Traefik) terminando TLS/repassando IP na frente do backend.
var trustedProxyNets []*net.IPNet

func SetTrustedProxies(nets []*net.IPNet) {
	trustedProxyNets = nets
}

func isTrustedPeer(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, n := range trustedProxyNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// isRequestHTTPS detecta se a requisição chegou por TLS — direto (r.TLS,
// raro já que a instalação padrão publica a porta do backend crua) ou via
// X-Forwarded-Proto, mas só quando o peer imediato (r.RemoteAddr) é um
// proxy confiável configurado — ver trustedProxyNets.
func isRequestHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return isTrustedPeer(r.RemoteAddr) && r.Header.Get("X-Forwarded-Proto") == "https"
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

// clientIP extrai o IP de origem pro throttle de login. RemoteAddr (peer
// real do socket TCP) é a fonte padrão — não é forjável por quem manda a
// requisição. X-Forwarded-For só é honrado quando o peer imediato é um
// proxy confiável configurado (ver trustedProxyNets); nesse caso lê a
// entrada MAIS À DIREITA do header (o hop que o próprio proxy confiável
// anexou), nunca a mais à esquerda — essa pode ter sido forjada pelo
// cliente original antes de chegar no proxy. Sem essa distinção, um
// atacante manda um X-Forwarded-For diferente a cada tentativa e o
// throttle por IP nunca acumula (achado de auditoria).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// Requisição autenticada pela chave de integração (Worker do mestre, via
	// cloudflared): a âncora de confiança é posse da chave, não posição de
	// rede — isTrustedPeer não se aplica (o IP do container cloudflared na
	// rede Docker interna não é algo que valha a pena fixar em
	// TRUSTED_PROXIES). CF-Connecting-IP é o header real que a borda da
	// Cloudflare injeta, não forjável por quem não seja a própria Cloudflare
	// nesse caminho (a request só chega aqui através do túnel outbound-only).
	if isIntegrationAuthed(r.Context()) {
		if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
			return cf
		}
		return host
	}
	if isTrustedPeer(r.RemoteAddr) {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			parts := strings.Split(fwd, ",")
			if last := strings.TrimSpace(parts[len(parts)-1]); last != "" {
				return last
			}
		}
	}
	return host
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	ip := clientIP(r)
	token, err := h.service.Login(r.Context(), req.Username, req.Password, ip, ip, r.UserAgent())
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

// ListSessions — quem está logado agora (tela de Gestão de sessões).
// requireAdmin no router: sessão/IP de OUTRO usuário é dado sensível,
// viewer não precisa enxergar.
func (h *AuthHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.service.ListActiveSessions(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	currentID := ""
	if sess, ok := auth.SessionFromContext(r.Context()); ok {
		currentID = sess.ID
	}
	for i := range sessions {
		sessions[i].Current = sessions[i].ID == currentID
	}
	httpx.WriteJSON(w, http.StatusOK, sessions)
}

// SessionHistory — log de sessão (ativa + revogada + expirada), mais
// recente primeiro.
func (h *AuthHandler) SessionHistory(w http.ResponseWriter, r *http.Request) {
	limit := parseIntDefault(r.URL.Query().Get("limit"), 200, 1, 1000)
	sessions, err := h.service.ListSessionHistory(r.Context(), limit)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	currentID := ""
	if sess, ok := auth.SessionFromContext(r.Context()); ok {
		currentID = sess.ID
	}
	for i := range sessions {
		sessions[i].Current = sessions[i].ID == currentID
	}
	httpx.WriteJSON(w, http.StatusOK, sessions)
}

// RevokeSession — botão "encerrar" na tela de Gestão de sessões, derruba
// qualquer sessão (inclusive de outro admin) na hora. Já é admin-only pela
// regra genérica do withAuth (DELETE = escrita).
func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RevokeSession(r.Context(), r.PathValue("id")); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// LoginAttempts — tentativa de login (sucesso e falha), mais recente
// primeiro. requireAdmin no router: IP/user-agent de tentativa alheia é
// dado sensível.
func (h *AuthHandler) LoginAttempts(w http.ResponseWriter, r *http.Request) {
	limit := parseIntDefault(r.URL.Query().Get("limit"), 200, 1, 1000)
	attempts, err := h.service.ListLoginAttempts(r.Context(), limit)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, attempts)
}
