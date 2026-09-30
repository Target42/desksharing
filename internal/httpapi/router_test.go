package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTenantEntryRoute(t *testing.T) {
	h := NewRouter(nil, nil, nil, nil)

	t.Run("serves login page", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/acme", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "form-login") {
			t.Fatal("expected login form")
		}
	})

	t.Run("redirects trailing slash and case", func(t *testing.T) {
		for _, path := range []string{"/acme/", "/Acme"} {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusPermanentRedirect {
				t.Fatalf("%s status %d", path, rec.Code)
			}
			if rec.Header().Get("Location") != "/acme" {
				t.Fatalf("%s location %q", path, rec.Header().Get("Location"))
			}
		}
	})

	t.Run("keeps app routes", func(t *testing.T) {
		for _, path := range []string{"/login", "/app", "/app/bookings"} {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("%s status %d", path, rec.Code)
			}
		}
	})

	t.Run("rejects reserved and invalid slugs", func(t *testing.T) {
		for _, path := range []string{"/foo_bar", "/favicon.ico", "/foo/bar"} {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s status %d, want 404", path, rec.Code)
			}
		}
	})
}
