package auth

import "testing"

func TestValidTenantSlug(t *testing.T) {
	ok := []string{"acme", "acme-gmbh", "a1"}
	for _, slug := range ok {
		if !ValidTenantSlug(slug) {
			t.Fatalf("expected %q to be valid", slug)
		}
	}

	bad := []string{"", "app", "login", "register", "api", "assets", "healthz", "invite", "verify", "a_b", "-acme", "acme-", "Acme", "foo/bar"}
	for _, slug := range bad {
		if ValidTenantSlug(slug) {
			t.Fatalf("expected %q to be rejected", slug)
		}
	}
}
