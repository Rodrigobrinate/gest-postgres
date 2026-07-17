package api

import (
	"net/http"

	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/server"
)

func (h *DetailHandler) SuggestIndexes(w http.ResponseWriter, r *http.Request) {
	suggestions, err := h.service.SuggestMissingIndexes(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if suggestions == nil {
		suggestions = []server.IndexSuggestion{}
	}
	httpx.WriteJSON(w, http.StatusOK, suggestions)
}

func (h *DetailHandler) UnusedIndexes(w http.ResponseWriter, r *http.Request) {
	unused, err := h.service.ListUnusedIndexes(r.Context(), r.PathValue("id"), r.URL.Query().Get("database"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if unused == nil {
		unused = []server.UnusedIndex{}
	}
	httpx.WriteJSON(w, http.StatusOK, unused)
}

func (h *DetailHandler) ReindexConcurrently(w http.ResponseWriter, r *http.Request) {
	err := h.service.ReindexConcurrently(
		r.Context(),
		r.PathValue("id"),
		r.URL.Query().Get("database"),
		r.PathValue("schema"),
		r.PathValue("name"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *DetailHandler) DropIndex(w http.ResponseWriter, r *http.Request) {
	err := h.service.DropIndex(
		r.Context(),
		r.PathValue("id"),
		r.URL.Query().Get("database"),
		r.PathValue("schema"),
		r.PathValue("name"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
