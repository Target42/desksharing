package httpapi

import (
	"encoding/json"
	"net/http"

	"desksharing/internal/auth"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	if e, ok := auth.AsError(err); ok {
		writeJSON(w, e.Status, map[string]string{
			"error":   e.Code,
			"message": e.Message,
		})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{
		"error":   "internal",
		"message": "internal server error",
	})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
