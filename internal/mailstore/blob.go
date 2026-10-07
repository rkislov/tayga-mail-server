package mailstore

import (
	"context"
	"errors"
)

var errBlobNotFound = errors.New("object not found")

// Blob is an optional durable object store behind the local maildir cache.
// Keys use forward-slash paths matching maildir relative paths (e.g. user@ex.com/new/…).
type Blob interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// MemBlob is an in-memory Blob for tests.
type MemBlob struct {
	m map[string][]byte
}

func NewMemBlob() *MemBlob {
	return &MemBlob{m: make(map[string][]byte)}
}

func (b *MemBlob) Put(_ context.Context, key string, data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	b.m[key] = cp
	return nil
}

func (b *MemBlob) Get(_ context.Context, key string) ([]byte, error) {
	data, ok := b.m[key]
	if !ok {
		return nil, errBlobNotFound
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

func (b *MemBlob) Delete(_ context.Context, key string) error {
	delete(b.m, key)
	return nil
}
