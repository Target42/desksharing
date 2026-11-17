package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAndroidAPKDownload(t *testing.T) {
	dir := t.TempDir()
	mobile := filepath.Join(dir, "mobile")
	if err := os.Mkdir(mobile, 0o750); err != nil {
		t.Fatal(err)
	}
	payload := []byte("apk-bytes")
	if err := os.WriteFile(filepath.Join(mobile, "other.apk"), payload, 0o640); err != nil {
		t.Fatal(err)
	}
	h := NewRouter(nil, nil, nil, nil, ServerOptions{DataDir: dir, PublicBaseURL: "http://localhost:8081"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/app/android", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"available":true`) {
		t.Fatalf("info status %d body %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/download/desksharing.apk", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("download status %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/vnd.android.package-archive" {
		t.Fatalf("content type %q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != string(payload) {
		t.Fatalf("body %q", rec.Body.String())
	}

	if err := os.WriteFile(filepath.Join(mobile, "second.apk"), []byte("nope"), 0o640); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/download/desksharing.apk", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("ambiguous apk status %d, want 404", rec.Code)
	}

	if err := os.WriteFile(filepath.Join(mobile, androidAPKName), []byte("canonical"), 0o640); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/download/desksharing.apk", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "canonical" {
		t.Fatalf("canonical status %d body %q", rec.Code, rec.Body.String())
	}
}

func TestAndroidAPKMissing(t *testing.T) {
	h := NewRouter(nil, nil, nil, nil, ServerOptions{DataDir: t.TempDir(), PublicBaseURL: "http://localhost:8081"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/app/android", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"available":false`) {
		t.Fatalf("info status %d body %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/download/desksharing.apk", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("download status %d, want 404", rec.Code)
	}
}
