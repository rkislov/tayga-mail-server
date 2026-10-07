package imapserver

import "testing"

func TestIdlePayloadRoundTrip(t *testing.T) {
	p := encodeIdlePayload("node-a", "u@ex.com", "INBOX")
	if p == "" {
		t.Fatal("empty payload")
	}
	n, e, m, ok := decodeIdlePayload(p)
	if !ok || n != "node-a" || e != "u@ex.com" || m != "INBOX" {
		t.Fatalf("got %q %q %q ok=%v", n, e, m, ok)
	}
}

func TestIdlePayloadRejectsTabs(t *testing.T) {
	if encodeIdlePayload("a\tb", "u@ex.com", "INBOX") != "" {
		t.Fatal("expected reject")
	}
	if _, _, _, ok := decodeIdlePayload("only-two\tparts"); ok {
		t.Fatal("expected decode fail")
	}
}
