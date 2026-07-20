package api

import (
	"log/slog"
	"net/http"
	"regexp"
	"strings"
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

// withCORS libera o frontend a chamar a API. Origin só é refletido se
// bater na allowlist (nunca "*", porque o cookie de sessão exige
// Access-Control-Allow-Credentials: true, e navegador nenhum aceita isso
// combinado com origin "*" — mas refletir QUALQUER Origin com credenciais é
// igualmente perigoso: libera qualquer site a ler resposta autenticada
// cross-origin, hoje só barrado por SameSite=Lax no cookie). A allowlist
// vem do IP/domínio detectado pelo setup.sh (ALLOWED_ORIGINS), já que essa é
// uma app self-hosted sem uma origem fixa conhecida de antemão.
func withCORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin := r.Header.Get("Origin"); origin != "" && allowed[origin] {
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
}

const sessionCookieName = "gestpg_session"

// publicPaths não passa por withAuth — login precisa ser alcançável sem
// sessão, e healthz é usado por health-check de infra (setup.sh, orquestrador).
var publicPaths = map[string]bool{
	"/api/v1/auth/login": true,
	"/api/v1/healthz":    true,
}

// gitWebhookPathRegex é a ÚNICA rota pública com segmento dinâmico no meio
// (o webhook de auto-deploy do Git — quem autentica ali é a assinatura HMAC
// do provedor, não sessão de usuário, ver internal/api/git_deployments.go).
// Casamento exato do padrão da rota, não por sufixo — sufixo tornaria
// pública qualquer rota futura que também termine em "/webhook".
var gitWebhookPathRegex = regexp.MustCompile(`^/api/v1/infra/git-deployments/[^/]+/webhook$`)

func isPublicPath(path string) bool {
	return publicPaths[path] || gitWebhookPathRegex.MatchString(path)
}

// selfServicePaths são POST/DELETE que qualquer sessão autenticada pode
// chamar mesmo sendo viewer (não é "escrita de dado da plataforma", é ação
// sobre a própria sessão).
var selfServicePaths = map[string]bool{
	"/api/v1/auth/logout": true,
}

// withAuth exige uma sessão válida (cookie httpOnly) pra qualquer rota fora
// da allowlist, e aplica a regra de papel: viewer só pode método de leitura
// (GET/HEAD) — qualquer escrita (POST/PUT/PATCH/DELETE) exige admin. Isso
// cobre a API inteira sem precisar marcar rota por rota (são ~150+), com
// UMA exceção: terminal (WebSocket, tecnicamente um GET) dá controle total
// do container, então é sempre admin-only mesmo sendo GET.
func withAuth(authService *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublicPath(r.URL.Path) {
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

			if !sess.IsAdmin() && !selfServicePaths[r.URL.Path] {
				isWrite := r.Method != http.MethodGet && r.Method != http.MethodHead
				isTerminal := strings.HasSuffix(r.URL.Path, "/exec")
				if isWrite || isTerminal {
					httpx.WriteError(w, http.StatusForbidden, "essa ação exige papel admin")
					return
				}
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

// requireAdmin é redundante com a regra de escrita do withAuth pra
// POST/DELETE, mas cobre também o GET de listar usuário — um viewer não
// precisa enxergar o roster de contas da plataforma.
func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := auth.SessionFromContext(r.Context())
		if !ok || !sess.IsAdmin() {
			httpx.WriteError(w, http.StatusForbidden, "essa ação exige papel admin")
			return
		}
		next.ServeHTTP(w, r)
	}
}
