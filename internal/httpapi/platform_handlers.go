package httpapi

import (
	"net/http"

	"desksharing/internal/auth"

	"github.com/google/uuid"
)

type PlatformHandlers struct {
	Service *auth.Service
}

func (h PlatformHandlers) ListTenants(w http.ResponseWriter, r *http.Request) {
	items, err := h.Service.ListPlatformTenants(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h PlatformHandlers) SetTenantStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, auth.ErrInvalidInput("id must be a UUID"))
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("Ungültiger JSON-Body"))
		return
	}
	item, err := h.Service.SetTenantStatus(r.Context(), id, body.Status)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
