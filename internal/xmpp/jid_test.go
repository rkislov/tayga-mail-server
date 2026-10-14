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

func TestNormalizeJIDProfiles(t *testing.T) {
	j, err := NormalizeJID("ALICE@EXAMPLE.com/Phone One")
	if err != nil || j.Full() != "alice@example.com/Phone One" {
		t.Fatalf("%+v %v", j, err)
	}
	j, err = NormalizeJID("alice@bücher.example/Phone")
	if err != nil || j.Domain != "xn--bcher-kva.example" {
		t.Fatalf("%+v %v", j, err)
	}
	for _, s := range []string{"@example.com", "alice@", "alice@example.com/", "ali ce@example.com", "ali<ce@example.com", "alice@example.com/\x00", "alice@bad domain"} {
		if j, err := NormalizeJID(s); err == nil {
			t.Fatalf("accepted invalid JID %q: %+v", s, j)
		}
	}
}
