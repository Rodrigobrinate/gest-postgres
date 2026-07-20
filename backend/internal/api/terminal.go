package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	"github.com/gest-postgres/backend/internal/infra"
)

type TerminalHandler struct {
	service *infra.Service
}

func NewTerminalHandler(service *infra.Service) *TerminalHandler {
	return &TerminalHandler{service: service}
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
// InsecureSkipVerify porque essa é uma app self-hosted sem origem fixa
// conhecida (mesmo raciocínio do CORS por reflexão de Origin em
// middleware.go) — o cookie de sessão é o controle de acesso real, não a
// checagem de Origin do WebSocket.
func (h *TerminalHandler) Exec(w http.ResponseWriter, r *http.Request) {
	containerID := r.PathValue("containerId")

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
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
