package auth

import "context"

type ctxKey struct{}

// WithSession anexa a sessão autenticada no contexto — chamado pelo
// middleware withAuth (internal/api/middleware.go) depois de validar o
// cookie, nunca diretamente por um handler.
func WithSession(ctx context.Context, sess *Session) context.Context {
	return context.WithValue(ctx, ctxKey{}, sess)
}

// SessionFromContext lê a sessão anexada por WithSession. Só existe em
// rotas atrás de withAuth — nunca chamar em rotas na allowlist
// (login/healthz).
func SessionFromContext(ctx context.Context) (*Session, bool) {
	sess, ok := ctx.Value(ctxKey{}).(*Session)
	return sess, ok
}
