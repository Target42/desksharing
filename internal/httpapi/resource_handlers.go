package httpapi

import (
	"net/http"

	"desksharing/internal/auth"
	"desksharing/internal/catalog"

	"github.com/google/uuid"
)

func (h CatalogHandlers) ListResourceGroups(w http.ResponseWriter, r *http.Request) {
	items, err := h.Service.ListResourceGroups(r.Context(), h.tenantID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []catalog.ResourceGroup{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h CatalogHandlers) CreateResourceGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	item, err := h.Service.CreateResourceGroup(r.Context(), h.tenantID(r), body.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h CatalogHandlers) DeleteResourceGroup(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.Service.DeleteResourceGroup(r.Context(), h.tenantID(r), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h CatalogHandlers) ListResources(w http.ResponseWriter, r *http.Request) {
	items, err := h.Service.ListResources(r.Context(), h.tenantID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []catalog.Resource{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h CatalogHandlers) CreateResource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GroupID     string `json:"group_id"`
		LocationID  string `json:"location_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Seats       *int   `json:"seats"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	groupID, err := uuid.Parse(body.GroupID)
	if err != nil {
		writeError(w, auth.ErrInvalidInput("group_id must be a UUID"))
		return
	}
	locationID, err := uuid.Parse(body.LocationID)
	if err != nil {
		writeError(w, auth.ErrInvalidInput("location_id must be a UUID"))
		return
	}
	item, err := h.Service.CreateResource(r.Context(), h.tenantID(r), catalog.CreateResourceInput{
		GroupID:     groupID,
		LocationID:  locationID,
		Name:        body.Name,
		Description: body.Description,
		Seats:       body.Seats,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h CatalogHandlers) DeleteResource(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.Service.DeleteResource(r.Context(), h.tenantID(r), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
