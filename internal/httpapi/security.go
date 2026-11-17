package httpapi

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookie     = "desksharing_session"
	hostSessionCookie = "__Host-desksharing_session"

	// Leaflet sets element styles, so style-src allows inline CSS.
	// Scripts stay on this origin. Map tiles and address search are the
	// only other hosts. frame-ancestors blocks embedding from sibling apps.
	contentSecurityPolicy = "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob: https://*.tile.openstreetmap.org; " +
		"font-src 'self'; " +
		"connect-src 'self' https://nominatim.openstreetmap.org; " +
		"worker-src 'self'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"object-src 'none'"
)

// ServerOptions configures cookie scope and client IP handling.
// Cookies stay host-only (no Domain attribute) so a sibling subdomain
// on the same server does not receive the session.
type ServerOptions struct {
	PublicBaseURL string
	SecureCookie  bool
	TrustProxy    bool
	SessionTTL    time.Duration
	DataDir       string
}

func securityHeaders(secure bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		// Host-only. includeSubDomains would also force HTTPS onto other apps
		// if this process were ever served from the parent domain.
		if secure {
			h.Set("Strict-Transport-Security", "max-age=15552000")
		}
		next.ServeHTTP(w, r)
	})
}

func originGuard(publicBase string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestAllowed(r, publicBase) {
			next.ServeHTTP(w, r)
			return
		}
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error":   "forbidden",
			"message": "Anfrage von einer anderen Website abgelehnt.",
		})
	})
}

func requestAllowed(r *http.Request, publicBase string) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		// same-site covers sibling subdomains. A bearer token is not sent
		// automatically, so API clients may call across sites.
		return bearerToken(r) != ""
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return bearerToken(r) != "" || readSessionCookie(r) == ""
	}
	if strings.EqualFold(origin, "null") {
		return false
	}
	return originAllowed(origin, r.Host, publicBase)
}

func originAllowed(origin, requestHost, publicBase string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	pub, err := url.Parse(publicBase)
	if err != nil {
		pub = nil
	}
	if pub != nil && pub.Scheme == "https" && u.Scheme != "https" {
		return false
	}
	if strings.EqualFold(u.Host, requestHost) {
		return true
	}
	return pub != nil && pub.Host != "" && strings.EqualFold(u.Host, pub.Host)
}

func tokenFromRequest(r *http.Request) string {
	if token := bearerToken(r); token != "" {
		return token
	}
	return readSessionCookie(r)
}

func readSessionCookie(r *http.Request) string {
	for _, name := range []string{hostSessionCookie, sessionCookie} {
		c, err := r.Cookie(name)
		if err == nil && c.Value != "" {
			return c.Value
		}
	}
	return ""
}

func setSessionCookie(w http.ResponseWriter, token string, secure bool, ttl time.Duration) {
	name := sessionCookie
	if secure {
		name = hostSessionCookie
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		Expires:  time.Now().Add(ttl),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookies(w http.ResponseWriter, secure bool) {
	for _, name := range []string{sessionCookie, hostSessionCookie} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			Secure:   secure || name == hostSessionCookie,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string][]time.Time
	calls   int
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{buckets: map[string][]time.Time{}}
}

func (l *rateLimiter) allow(key string, limit int, window time.Duration) bool {
	now := time.Now()
	cutoff := now.Add(-window)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	if l.calls%100 == 0 {
		l.sweep(cutoff)
	}
	hits := l.buckets[key]
	kept := hits[:0]
	for _, ts := range hits {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= limit {
		l.buckets[key] = kept
		return false
	}
	l.buckets[key] = append(kept, now)
	return true
}

func (l *rateLimiter) sweep(cutoff time.Time) {
	for key, hits := range l.buckets {
		fresh := false
		for _, ts := range hits {
			if ts.After(cutoff) {
				fresh = true
				break
			}
		}
		if !fresh {
			delete(l.buckets, key)
		}
	}
}

func (l *rateLimiter) wrap(name string, limit int, window time.Duration, trustProxy bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := name + "|" + clientIP(r, trustProxy)
		if !l.allow(key, limit, window) {
			w.Header().Set("Retry-After", "60")
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error":   "rate_limited",
				"message": "Zu viele Versuche. Bitte kurz warten.",
			})
			return
		}
		next(w, r)
	}
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(ip) != nil {
			return ip
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			ip := strings.TrimSpace(parts[len(parts)-1])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
