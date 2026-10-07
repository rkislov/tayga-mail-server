package auth

import (
	"sync"
	"time"
)

type cacheEntry struct {
	ok        bool
	userID    string
	expiresAt time.Time
}

// authCache is a simple in-memory TTL cache for LDAP auth results.
type authCache struct {
	mu     sync.RWMutex
	ttl    time.Duration
	negTTL time.Duration
	items  map[string]cacheEntry
}

func newAuthCache(ttl time.Duration) *authCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	neg := ttl / 10
	if neg < 30*time.Second {
		neg = 30 * time.Second
	}
	if neg > ttl {
		neg = ttl
	}
	return &authCache{
		ttl:    ttl,
		negTTL: neg,
		items:  make(map[string]cacheEntry),
	}
}

func cacheKey(email, password string) string {
	// Bind password into key so password changes invalidate cache.
	// Not stored in cleartext beyond process memory of the map key.
	return email + "\x00" + password
}

func (c *authCache) get(email, password string) (userID string, ok bool, hit bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, exists := c.items[cacheKey(email, password)]
	if !exists || time.Now().After(e.expiresAt) {
		return "", false, false
	}
	return e.userID, e.ok, true
}

func (c *authCache) setOK(email, password string, userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[cacheKey(email, password)] = cacheEntry{
		ok:        true,
		userID:    userID,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *authCache) setFail(email, password string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[cacheKey(email, password)] = cacheEntry{
		ok:        false,
		expiresAt: time.Now().Add(c.negTTL),
	}
}
