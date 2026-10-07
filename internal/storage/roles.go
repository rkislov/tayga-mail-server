package storage

import "strings"

const RoleAdmin = "admin"

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

// HasRole reports whether roles contains role.
func HasRole(roles, role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	for _, r := range ParseRoles(roles) {
		if r == role {
			return true
		}
	}
	return false
}

// WithRole returns roles with role added.
func WithRole(roles, role string) string {
	list := ParseRoles(roles)
	role = strings.ToLower(strings.TrimSpace(role))
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
	var out []string
	for _, r := range ParseRoles(roles) {
		if r != role {
			out = append(out, r)
		}
	}
	return JoinRoles(out)
}
