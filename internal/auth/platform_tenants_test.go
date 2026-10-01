package auth

import "testing"

func TestTenantStatusChange(t *testing.T) {
	allowed := [][2]string{
		{"pending", "active"},
		{"pending", "disabled"},
		{"active", "disabled"},
		{"disabled", "active"},
		{"active", "active"},
		{"disabled", "disabled"},
	}
	for _, pair := range allowed {
		if err := tenantStatusChange(pair[0], pair[1]); err != nil {
			t.Fatalf("%s -> %s: %v", pair[0], pair[1], err)
		}
	}
	denied := [][2]string{
		{"active", "pending"},
		{"disabled", "pending"},
		{"pending", "pending"},
		{"active", "archived"},
	}
	for _, pair := range denied {
		if err := tenantStatusChange(pair[0], pair[1]); err == nil {
			t.Fatalf("%s -> %s accepted", pair[0], pair[1])
		}
	}
}
