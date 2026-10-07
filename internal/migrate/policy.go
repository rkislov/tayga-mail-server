package migrate

import (
	"context"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

// CoSFeatures is the subset of CoS config used for migration policy.
type CoSFeatures map[string]bool

// ResolveAllowed applies user → domain → CoS → default false.
func ResolveAllowed(userPolicy, domainPolicy string, cosFeatures CoSFeatures) bool {
	if v, ok := parseOverride(userPolicy); ok {
		return v
	}
	if v, ok := parseOverride(domainPolicy); ok {
		return v
	}
	if cosFeatures != nil {
		if v, ok := cosFeatures["migration"]; ok {
			return v
		}
	}
	return false
}

func parseOverride(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on", "true", "1", "enabled":
		return true, true
	case "off", "false", "0", "disabled":
		return false, true
	default:
		return false, false // inherit
	}
}

// ResolveForUser loads domain + returns whether migration UI/API is allowed.
// cosFeatures comes from the caller's CoS parser.
func ResolveForUser(ctx context.Context, store storage.Driver, u *storage.User, cosFeatures CoSFeatures) bool {
	if u == nil {
		return false
	}
	domainPolicy := "inherit"
	if d, err := store.GetDomainByID(ctx, u.DomainID); err == nil && d != nil {
		domainPolicy = d.MigrationEnabled
	}
	return ResolveAllowed(u.MigrationEnabled, domainPolicy, cosFeatures)
}
