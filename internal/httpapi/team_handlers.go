package httpapi

import (
	"net/http"
	"strings"

	"desksharing/internal/auth"

	"github.com/google/uuid"
)

type TeamHandlers struct {
	Service *auth.Service
}

func (h TeamHandlers) List(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	items, err := h.Service.ListTeams(r.Context(), *user.TenantID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h TeamHandlers) ListMine(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	items, err := h.Service.ListMyTeams(r.Context(), *user.TenantID, user.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h TeamHandlers) Create(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("Ungültiger JSON-Body"))
		return
	}
	item, err := h.Service.CreateTeam(r.Context(), *user.TenantID, body.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h TeamHandlers) Rename(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	id, err := teamID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("Ungültiger JSON-Body"))
		return
	}
	item, err := h.Service.RenameTeam(r.Context(), *user.TenantID, id, body.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h TeamHandlers) SetMembers(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	id, err := teamID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		UserIDs []string `json:"user_ids"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("Ungültiger JSON-Body"))
		return
	}
	ids := make([]uuid.UUID, 0, len(body.UserIDs))
	for _, raw := range body.UserIDs {
		parsed, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			writeError(w, auth.ErrInvalidInput("user_ids müssen UUIDs sein"))
			return
		}
		ids = append(ids, parsed)
	}
	item, err := h.Service.SetTeamMembers(r.Context(), *user.TenantID, id, ids)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h TeamHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	id, err := teamID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.Service.DeleteTeam(r.Context(), *user.TenantID, id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func teamID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, auth.ErrInvalidInput("id must be a UUID")
	}
	return id, nil
}
