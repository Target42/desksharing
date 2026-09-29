package httpapi

import (
	"net/http"

	"desksharing/internal/auth"

	"github.com/google/uuid"
)

type MemberHandlers struct {
	Service *auth.Service
}

func (h MemberHandlers) List(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	items, err := h.Service.ListMembers(r.Context(), *user.TenantID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h MemberHandlers) Create(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	var in auth.CreateMemberInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, auth.ErrInvalidInput("Ungültiger JSON-Body"))
		return
	}
	out, err := h.Service.CreateMember(r.Context(), *user.TenantID, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h MemberHandlers) Import(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeError(w, auth.ErrInvalidInput("Die Datei ist zu groß oder ungültig (max. 2 MB)."))
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeError(w, auth.ErrInvalidInput("Datei ist erforderlich"))
		return
	}
	defer file.Close()

	out, err := h.Service.ImportMembers(r.Context(), *user.TenantID, hdr.Filename, file)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h MemberHandlers) Template(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	switch format {
	case "xlsx":
		data, err := auth.MemberImportTemplateXLSX()
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="mitarbeiter-vorlage.xlsx"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	default:
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="mitarbeiter-vorlage.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(auth.MemberImportTemplateCSV())
	}
}

func (h MemberHandlers) ResendInvite(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	id, err := memberID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.Service.ResendInvite(r.Context(), *user.TenantID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h MemberHandlers) SetDisabled(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	id, err := memberID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Disabled bool `json:"disabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("Ungültiger JSON-Body"))
		return
	}
	item, err := h.Service.SetMemberDisabled(r.Context(), *user.TenantID, user.ID, id, body.Disabled)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func memberID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, auth.ErrInvalidInput("id must be a UUID")
	}
	return id, nil
}
