package mailsearch

import "testing"

func TestParseQuery(t *testing.T) {
	q := ParseQuery(`from:alice@example.com subject:hello world body`)
	if q.From != "alice@example.com" {
		t.Fatalf("from=%q", q.From)
	}
	if q.Subject != "hello" {
		t.Fatalf("subject=%q", q.Subject)
	}
	if q.FTSQuery() != "world body" {
		t.Fatalf("fts=%q", q.FTSQuery())
	}
}

func TestParseDocumentPlain(t *testing.T) {
	raw := []byte("From: a@b.c\r\nTo: d@e.f\r\nSubject: Hi\r\nDate: Mon, 1 Jan 2024 00:00:00 +0000\r\nContent-Type: text/plain\r\n\r\nHello World\r\n")
	doc := ParseDocument(raw)
	if doc.Subject != "Hi" || doc.From != "a@b.c" || doc.To != "d@e.f" {
		t.Fatalf("headers: %+v", doc)
	}
	if doc.Body != "hello world" {
		t.Fatalf("body=%q", doc.Body)
	}
}
