package smtp

import (
	"sync"
	"time"
)

// rateLimiter is a simple token-bucket style fixed-window counter.
type rateLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	limit   int
	hits    map[string]bucket
}

type bucket struct {
	n    int
	from time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	if limit <= 0 || window <= 0 {
		return nil
	}
	return &rateLimiter{
		window: window,
		limit:  limit,
		hits:   make(map[string]bucket),
	}
}

// Allow returns false when the key has exceeded the limit in the current window.
func (r *rateLimiter) Allow(key string) bool {
	if r == nil || key == "" {
		return true
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.hits[key]
	if !ok || now.Sub(b.from) >= r.window {
		r.hits[key] = bucket{n: 1, from: now}
		r.gc(now)
		return true
	}
	if b.n >= r.limit {
		return false
	}
	b.n++
	r.hits[key] = b
	return true
}

func (r *rateLimiter) gc(now time.Time) {
	if len(r.hits) < 1024 {
		return
	}
	for k, b := range r.hits {
		if now.Sub(b.from) >= r.window*2 {
			delete(r.hits, k)
		}
	}
}
