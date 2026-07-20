package api

import (
	"net/http"

	"github.com/gest-postgres/backend/internal/auth"
	"github.com/gest-postgres/backend/internal/httpx"
)

type UsersHandler struct {
	service *auth.Service
}

func NewUsersHandler(service *auth.Service) *UsersHandler {
	return &UsersHandler{service: service}
}

func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.ListUsers(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, users)
}

type createUserInput struct {
	Username string    `json:"username"`
	Password string    `json:"password"`
	Role     auth.Role `json:"role"`
}

func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in createUserInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	user, err := h.service.CreateUser(r.Context(), in.Username, in.Password, in.Role)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, user)
}

func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	sess, ok := auth.SessionFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	if err := h.service.DeleteUser(r.Context(), sess.UserID, r.PathValue("userId")); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type resetPasswordInput struct {
	Password string `json:"password"`
}

func (h *UsersHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var in resetPasswordInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}
	if err := h.service.ResetPassword(r.Context(), r.PathValue("userId"), in.Password); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
