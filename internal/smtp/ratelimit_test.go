package smtp

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	r := newRateLimiter(2, time.Minute)
	if !r.Allow("a") || !r.Allow("a") {
		t.Fatal("first two should pass")
	}
	if r.Allow("a") {
		t.Fatal("third should fail")
	}
	if !r.Allow("b") {
		t.Fatal("other key ok")
	}
	if newRateLimiter(0, time.Minute) != nil {
		t.Fatal("zero limit disables")
	}
}