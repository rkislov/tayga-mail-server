package smtp

import (
	"testing"
	"time"
)

func TestGreylistDeferThenPass(t *testing.T) {
	p := &greylistPolicy{
		delay: 50 * time.Millisecond, passTTL: time.Hour, ipv4Net: 32,
		entries: map[string]*greyEntry{},
	}
	if err := p.check("203.0.113.1", "a@ex.com", "b@ex.com"); err == nil {
		t.Fatal("expected defer")
	}
	if err := p.check("203.0.113.1", "a@ex.com", "b@ex.com"); err == nil {
		t.Fatal("still too early")
	}
	time.Sleep(60 * time.Millisecond)
	if err := p.check("203.0.113.1", "a@ex.com", "b@ex.com"); err != nil {
		t.Fatal(err)
	}
	if err := p.check("203.0.113.1", "a@ex.com", "b@ex.com"); err != nil {
		t.Fatal(err)
	}
}

func TestGreylistIPv4Net(t *testing.T) {
	if greylistKey("203.0.113.10", "a@e", "b@e", 24) != greylistKey("203.0.113.99", "a@e", "b@e", 24) {
		t.Fatal("same /24 should share key")
	}
	if greylistKey("203.0.113.10", "a@e", "b@e", 32) == greylistKey("203.0.113.99", "a@e", "b@e", 32) {
		t.Fatal("exact IP should differ")
	}
}

func TestGreylistCleanup(t *testing.T) {
	p := &greylistPolicy{passTTL: time.Minute, entries: map[string]*greyEntry{}}
	p.entries["x"] = &greyEntry{lastSeen: time.Now().UTC().Add(-2 * time.Hour)}
	p.cleanup(time.Now().UTC())
	if len(p.entries) != 0 {
		t.Fatal("expected cleanup")
	}
}
