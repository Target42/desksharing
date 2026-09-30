package auth

import "regexp"

// tenantSlugPattern matches a single URL path segment used as the login entry,
// for example /acme. The slug is not part of later app paths.
var tenantSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// reservedTenantSlugs are path segments owned by the app itself.
var reservedTenantSlugs = map[string]struct{}{
	"api":      {},
	"app":      {},
	"assets":   {},
	"healthz":  {},
	"invite":   {},
	"login":    {},
	"register": {},
	"verify":   {},
}

// ValidTenantSlug reports whether slug can be used as the login path /{slug}.
func ValidTenantSlug(slug string) bool {
	if !tenantSlugPattern.MatchString(slug) {
		return false
	}
	_, reserved := reservedTenantSlugs[slug]
	return !reserved
}
