package storage

import "strings"

// DisplayLabel never substitutes generated protocol identifiers for a name.
func (u *User) DisplayLabel() string {
	if value := strings.TrimSpace(u.DisplayName); value != "" {
		return value
	}
	if value := strings.TrimSpace(u.Email); value != "" {
		return value
	}
	return strings.TrimSpace(u.LocalPart)
}
