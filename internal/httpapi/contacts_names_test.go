package httpapi

import "testing"

func TestContactStructuredNames(t *testing.T) {
	c := parseSimpleVCard("BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Anna Smith\r\nN:Smith;Anna;Marie;;\r\nEND:VCARD\r\n")
	if c.FirstName != "Anna" || c.LastName != "Smith" || c.MiddleName != "Marie" || c.FN != "Anna Smith" {
		t.Fatalf("names: %+v", c)
	}
	c = parseSimpleVCard("BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Company Support\r\nEND:VCARD\r\n")
	if c.FirstName != "" || c.LastName != "" || c.FN != "Company Support" {
		t.Fatalf("invented name parts: %+v", c)
	}
}
