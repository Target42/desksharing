package httpapi

import (
	"net/http"
	"time"

	"desksharing/internal/auth"
	"desksharing/internal/booking"

	"github.com/google/uuid"
)

func (h BookingHandlers) ListBlocks(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := h.tenantUser(r)
	items, err := h.Service.ListBlocks(r.Context(), tenantID)
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []booking.Block{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h BookingHandlers) CreateBlocks(w http.ResponseWriter, r *http.Request) {
	tenantID, userID := h.tenantUser(r)
	var body struct {
		PlaceIDs []string `json:"place_ids"`
		RoomID   string   `json:"room_id"`
		StartsAt string   `json:"starts_at"`
		EndsAt   string   `json:"ends_at"`
		Reason   string   `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("Ungültiger JSON-Body"))
		return
	}
	placeIDs := make([]uuid.UUID, 0, len(body.PlaceIDs))
	for _, raw := range body.PlaceIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, auth.ErrInvalidInput("Ungültige Platz-ID"))
			return
		}
		placeIDs = append(placeIDs, id)
	}
	in := booking.CreateBlocksInput{
		PlaceIDs: placeIDs,
		Reason:   body.Reason,
	}
	if body.RoomID != "" {
		id, err := uuid.Parse(body.RoomID)
		if err != nil {
			writeError(w, auth.ErrInvalidInput("Ungültige Raum-ID"))
			return
		}
		in.RoomID = &id
	}
	var err error
	in.StartsAt, err = time.Parse(time.RFC3339, body.StartsAt)
	if err != nil {
		writeError(w, auth.ErrInvalidInput("Beginn ist ungültig"))
		return
	}
	in.EndsAt, err = time.Parse(time.RFC3339, body.EndsAt)
	if err != nil {
		writeError(w, auth.ErrInvalidInput("Ende ist ungültig"))
		return
	}
	items, err := h.Service.CreateBlocks(r.Context(), tenantID, userID, in)
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []booking.Block{}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"items": items})
}

func (h BookingHandlers) DeleteBlock(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.Service.DeleteBlock(r.Context(), *user.TenantID, user.ID, id, user.Role == "tenant_admin"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
