package httpapi

import (
	"net/http"

	"desksharing/internal/auth"
)

type AuthHandlers struct {
	Service *auth.Service
}

func (h AuthHandlers) RegisterTenant(w http.ResponseWriter, r *http.Request) {
	var in auth.RegisterTenantInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	out, err := h.Service.RegisterTenant(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h AuthHandlers) Verify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		var body struct {
			Token string `json:"token"`
		}
		_ = decodeJSON(r, &body)
		token = body.Token
	}
	if err := h.Service.VerifyToken(r.Context(), token); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (h AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var in auth.LoginInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	out, err := h.Service.Login(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	_ = h.Service.Logout(r.Context(), bearerToken(r))
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (h AuthHandlers) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("Ungültiger JSON-Body"))
		return
	}
	if err := h.Service.AcceptInvite(r.Context(), body.Token, body.Password); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

func (h AuthHandlers) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrUnauthorized("missing user"))
		return
	}
	writeJSON(w, http.StatusOK, user)
}
