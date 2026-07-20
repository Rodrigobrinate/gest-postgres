package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	"github.com/gest-postgres/backend/internal/infra"
)

type TerminalHandler struct {
	service        *infra.Service
	allowedOrigins []string
}

func NewTerminalHandler(service *infra.Service, allowedOrigins []string) *TerminalHandler {
	return &TerminalHandler{service: service, allowedOrigins: allowedOrigins}
}

// termClientMessage é o envelope JSON que o frontend manda pro terminal —
// texto de entrada do teclado ou um resize do pseudo-terminal. Saída do
// container vai na direção contrária como frame binário cru (sem envelope),
// já que exec com Tty mescla stdout+stderr num stream só.
type termClientMessage struct {
	Type string `json:"type"` // "stdin" | "resize"
	Data string `json:"data,omitempty"`
	Cols uint   `json:"cols,omitempty"`
	Rows uint   `json:"rows,omitempty"`
}

// Exec abre um terminal interativo dentro do container via WebSocket. Fica
// atrás de withAuth como qualquer outra rota — o handshake do WebSocket é
// um GET normal, carrega o cookie de sessão automaticamente (same-site).
// OriginPatterns usa a mesma allowlist do CORS (ALLOWED_ORIGINS) em vez de
// InsecureSkipVerify — esse é o endpoint mais perigoso da API (shell
// interativo em qualquer container), então checar Origin é defesa em
// profundidade real, não perfumaria: sem isso, uma página cross-site
// conseguiria tentar abrir esse WS contando só com o SameSite=Lax do cookie.
func (h *TerminalHandler) Exec(w http.ResponseWriter, r *http.Request) {
	containerID := r.PathValue("containerId")

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.allowedOrigins})
	if err != nil {
		slog.Error("falha no upgrade do terminal", "error", err, "container_id", containerID)
		return
	}
	defer conn.CloseNow()

	ctx := r.Context()

	execID, hijacked, err := h.service.OpenTerminal(ctx, containerID)
	if err != nil {
		slog.Error("falha ao abrir terminal", "error", err, "container_id", containerID)
		conn.Close(websocket.StatusInternalError, "falha ao abrir terminal")
		return
	}
	defer hijacked.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, readErr := hijacked.Reader.Read(buf)
			if n > 0 {
				if writeErr := conn.Write(ctx, websocket.MessageBinary, buf[:n]); writeErr != nil {
					return
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	for {
		_, data, readErr := conn.Read(ctx)
		if readErr != nil {
			break
		}
		var msg termClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "stdin":
			if _, writeErr := hijacked.Conn.Write([]byte(msg.Data)); writeErr != nil {
				return
			}
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 {
				_ = h.service.ResizeTerminal(ctx, execID, msg.Rows, msg.Cols)
			}
		}
	}

	<-done
	conn.Close(websocket.StatusNormalClosure, "")
}
