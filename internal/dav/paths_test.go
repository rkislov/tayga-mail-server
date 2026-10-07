package dav

import "testing"

func TestParseCalPath(t *testing.T) {
	email, cal, href, ok := parseCalPath("/dav/cal/admin@example.com/calendars/default/evt.ics")
	if !ok || email != "admin@example.com" || cal != "default" || href != "evt.ics" {
		t.Fatalf("got email=%q cal=%q href=%q ok=%v", email, cal, href, ok)
	}
	email, cal, href, ok = parseCalPath("/dav/cal/admin@example.com/")
	if !ok || email != "admin@example.com" || cal != "" || href != "" {
		t.Fatalf("principal: email=%q cal=%q href=%q ok=%v", email, cal, href, ok)
	}
}

func TestParseCardPath(t *testing.T) {
	email, ab, href, ok := parseCardPath("/dav/card/admin@example.com/addressbooks/default/c.vcf")
	if !ok || email != "admin@example.com" || ab != "default" || href != "c.vcf" {
		t.Fatalf("got email=%q ab=%q href=%q ok=%v", email, ab, href, ok)
	}
}
