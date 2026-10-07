package storage

import "strings"

// Legacy and current admin role names.
const (
	RoleAdmin       = "admin" // legacy; treated as global_admin
	RoleGlobalAdmin = "global_admin"
	RoleDomainAdmin = "domain_admin"
	RoleUser        = "user"
)

// ParseRoles splits a stored roles string into normalized unique role names.
func ParseRoles(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	seen := map[string]struct{}{}
	var out []string
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if p == RoleAdmin {
			p = RoleGlobalAdmin
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// JoinRoles serializes roles for storage.
func JoinRoles(roles []string) string {
	return strings.Join(ParseRoles(strings.Join(roles, ",")), ",")
}

// HasRole reports whether roles contains role (admin ≡ global_admin).
func HasRole(roles, role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == RoleAdmin {
		role = RoleGlobalAdmin
	}
	for _, r := range ParseRoles(roles) {
		if r == role {
			return true
		}
	}
	return false
}

// IsGlobalAdmin reports global admin privilege.
func IsGlobalAdmin(roles string) bool {
	return HasRole(roles, RoleGlobalAdmin)
}

// IsDomainAdmin reports domain or global admin privilege.
func IsDomainAdmin(roles string) bool {
	return IsGlobalAdmin(roles) || HasRole(roles, RoleDomainAdmin)
}

// AdminScope returns "global", "domain", or "none".
func AdminScope(roles string) string {
	if IsGlobalAdmin(roles) {
		return "global"
	}
	if HasRole(roles, RoleDomainAdmin) {
		return "domain"
	}
	return "none"
}

// WithRole returns roles with role added.
func WithRole(roles, role string) string {
	list := ParseRoles(roles)
	role = strings.ToLower(strings.TrimSpace(role))
	if role == RoleAdmin {
		role = RoleGlobalAdmin
	}
	if role == "" {
		return JoinRoles(list)
	}
	for _, r := range list {
		if r == role {
			return JoinRoles(list)
		}
	}
	return JoinRoles(append(list, role))
}

// WithoutRole returns roles with role removed.
func WithoutRole(roles, role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == RoleAdmin {
		role = RoleGlobalAdmin
	}
	var out []string
	for _, r := range ParseRoles(roles) {
		if r != role {
			out = append(out, r)
		}
	}
	return JoinRoles(out)
}
