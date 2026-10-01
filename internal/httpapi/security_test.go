package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSecurityHeadersAndSiblingOrigin(t *testing.T) {
	h := NewRouter(nil, nil, nil, nil, ServerOptions{PublicBaseURL: "https://desk.example.com"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'self'") {
		t.Fatalf("csp %q", got)
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing frame denial")
	}
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("https site should send HSTS")
	}
	if strings.Contains(rec.Header().Get("Strict-Transport-Security"), "includeSubDomains") {
		t.Fatal("HSTS must not cover other apps on the parent domain")
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("API must not allow other origins to read responses")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	req.Host = "desk.example.com"
	req.Header.Set("Origin", "https://other.example.com")
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("sibling origin status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestVerifyIsNotAGet(t *testing.T) {
	h := NewRouter(nil, nil, nil, nil, ServerOptions{PublicBaseURL: "http://localhost:8081"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/verify?token=secret", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatal("token leaked in response")
	}
}

func TestSessionCookieIsHostOnly(t *testing.T) {
	rec := httptest.NewRecorder()
	setSessionCookie(rec, "tok", true, time.Hour)
	res := rec.Result()
	defer res.Body.Close()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies %#v", cookies)
	}
	c := cookies[0]
	if c.Name != hostSessionCookie || !c.HttpOnly || !c.Secure || c.Domain != "" || c.Path != "/" {
		t.Fatalf("cookie %#v", c)
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("samesite %v", c.SameSite)
	}
}

func TestRateLimiter(t *testing.T) {
	lim := newRateLimiter()
	for i := 0; i < 3; i++ {
		if !lim.allow("login|1.2.3.4", 3, time.Minute) {
			t.Fatalf("blocked at %d", i)
		}
	}
	if lim.allow("login|1.2.3.4", 3, time.Minute) {
		t.Fatal("expected limit")
	}
	if !lim.allow("login|5.6.7.8", 3, time.Minute) {
		t.Fatal("other client should be independent")
	}
}

func TestClientIPIgnoresForwardedHeadersUnlessTrusted(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.8:1234"
	req.Header.Set("X-Real-IP", "203.0.113.9")
	if got := clientIP(req, false); got != "10.0.0.8" {
		t.Fatalf("untrusted got %s", got)
	}
	if got := clientIP(req, true); got != "203.0.113.9" {
		t.Fatalf("trusted got %s", got)
	}
}
