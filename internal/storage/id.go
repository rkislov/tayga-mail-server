package storage

import "github.com/google/uuid"

// NewID returns a new random UUID string (RFC 4122 v4).
func NewID() string {
	return uuid.NewString()
}
