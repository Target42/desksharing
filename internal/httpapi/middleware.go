package httpapi

import (
	"context"
	"net/http"
	"strings"

	"desksharing/internal/auth"
)

type contextKey string

const userContextKey contextKey = "user"

func UserFromContext(ctx context.Context) (*auth.User, bool) {
	u, ok := ctx.Value(userContextKey).(*auth.User)
	return u, ok
}

func withUser(ctx context.Context, user *auth.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func RequireAuth(svc *auth.Service, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := svc.Authenticate(r.Context(), bearerToken(r))
		if err != nil {
			writeError(w, err)
			return
		}
		next(w, r.WithContext(withUser(r.Context(), user)))
	}
}

func RequireTenant(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok || user.TenantID == nil {
			writeError(w, auth.ErrForbidden("tenant context required"))
			return
		}
		next(w, r)
	}
}

func RequireTenantAdmin(next http.HandlerFunc) http.HandlerFunc {
	return RequireTenant(func(w http.ResponseWriter, r *http.Request) {
		user, _ := UserFromContext(r.Context())
		if user.Role != "tenant_admin" {
			writeError(w, auth.ErrForbidden("tenant admin required"))
			return
		}
		next(w, r)
	})
}

func withAuth(svc *auth.Service, roles ...func(http.HandlerFunc) http.HandlerFunc) func(http.HandlerFunc) http.HandlerFunc {
	return func(final http.HandlerFunc) http.HandlerFunc {
		h := final
		for i := len(roles) - 1; i >= 0; i-- {
			h = roles[i](h)
		}
		return RequireAuth(svc, h)
	}
}
