package xmpp

import "testing"

func TestParseJID(t *testing.T) {
	j := ParseJID("Alice@Example.COM/phone")
	if j.Local != "alice" || j.Domain != "example.com" || j.Resource != "phone" {
		t.Fatalf("got %+v", j)
	}
	if j.Bare() != "alice@example.com" {
		t.Fatalf("bare %q", j.Bare())
	}
	if j.Full() != "alice@example.com/phone" {
		t.Fatalf("full %q", j.Full())
	}
}
