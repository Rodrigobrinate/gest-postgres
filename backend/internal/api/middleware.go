package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gest-postgres/backend/internal/auth"
	"github.com/gest-postgres/backend/internal/httpx"
)

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// withCORS libera o frontend a chamar a API. Origin é refletido (nunca "*")
// porque o cookie de sessão exige Access-Control-Allow-Credentials: true, e
// navegador nenhum aceita isso combinado com origin "*". Sem allowlist fixa
// de origem porque essa é uma app self-hosted — o admin escolhe IP/domínio
// na hora do deploy (ver PUBLIC_API_URL no setup.sh), não tem uma origem
// única conhecida de antemão.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

const sessionCookieName = "gestpg_session"

// publicPaths não passa por withAuth — login precisa ser alcançável sem
// sessão, e healthz é usado por health-check de infra (setup.sh, orquestrador).
var publicPaths = map[string]bool{
	"/api/v1/auth/login": true,
	"/api/v1/healthz":    true,
}

// withAuth exige uma sessão válida (cookie httpOnly) pra qualquer rota fora
// da allowlist. Antes dessa mudança a API inteira respondia sem
// autenticação nenhuma — ver CLAUDE.md, item MVP "Login/senha" que estava
// pendente. Precisa ficar por DENTRO de withCORS na cadeia (withCORS chama
// next só depois de tratar OPTIONS) senão todo preflight do browser toma 401
// antes de chegar aqui.
func withAuth(authService *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if publicPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "não autenticado")
				return
			}
			sess, err := authService.ValidateSession(r.Context(), cookie.Value)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "sessão inválida ou expirada")
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithSession(r.Context(), sess)))
		})
	}
}

// requireElevated exige reconfirmação de senha recente (POST
// /api/v1/auth/step-up) na sessão atual — aplicado só nas rotas de
// escrita/exclusão do file manager do host, nunca globalmente. withAuth já
// deve ter rodado antes (rota tem que estar registrada atrás dele).
func requireElevated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := auth.SessionFromContext(r.Context())
		if !ok || !sess.Elevated() {
			httpx.WriteError(w, http.StatusForbidden, "confirme a senha de novo pra continuar (zona de risco)")
			return
		}
		next.ServeHTTP(w, r)
	}
}
