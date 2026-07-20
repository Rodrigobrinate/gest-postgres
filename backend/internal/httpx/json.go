// Package httpx tem helpers pequenos de request/response JSON usados por todos
// os handlers da API. Sem framework — net/http (Go 1.22+) já dá roteamento
// com método+path patterns, não precisa de chi/gin pra esse tamanho de API.
package httpx

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

// maxJSONBodyBytes limita o corpo de qualquer requisição JSON da API — sem
// isso um corpo gigante decodifica inteiro em memória antes de qualquer
// validação de campo rodar (DoS de memória). 10MB cobre com folga o maior
// payload legítimo hoje (texto de query SQL, YAML de compose, etc).
const maxJSONBodyBytes = 10 << 20

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("falha ao serializar resposta JSON", "error", err)
	}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}

func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSONBodyBytes))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
