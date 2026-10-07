package smtp

import (
	"strings"
	"testing"

	"github.com/emersion/go-msgauth/dmarc"
)

func TestShouldSendFailureReport(t *testing.T) {
	if !shouldSendFailureReport(0, false, false) {
		t.Fatal("default fo=0 should report when both fail")
	}
	if shouldSendFailureReport(dmarc.FailureAll, true, false) {
		t.Fatal("fo=0 should not report when one aligns")
	}
	if !shouldSendFailureReport(dmarc.FailureAny, true, false) {
		t.Fatal("fo=1 should report when one fails")
	}
	if !shouldSendFailureReport(dmarc.FailureSPF, false, true) {
		t.Fatal("fo=s should report SPF fail")
	}
	if !shouldSendFailureReport(dmarc.FailureDKIM, true, false) {
		t.Fatal("fo=d should report DKIM fail")
	}
}

func TestBuildFailureMessage(t *testing.T) {
	ev := &dmarcEvent{
		Domain: "example.com", SourceIP: "1.2.3.4",
		EnvelopeFrom: "a@evil.test", EnvelopeDomain: "evil.test",
		SPFResult: "fail", DKIMResult: "none", Disposition: "reject",
	}
	orig := []byte("From: a@example.com\r\nSubject: hi\r\n\r\nbody\r\n")
	msg, err := buildFailureMessage("dmarc@mail.test", []string{"ruf@example.com"}, ev, "mail.test; dmarc=fail", orig)
	if err != nil {
		t.Fatal(err)
	}
	s := string(msg)
	for _, want := range []string{
		"Auth-Failure: dmarc",
		"Reported-Domain: example.com",
		"Source-IP: 1.2.3.4",
		"Original-Mail-From: <a@evil.test>",
		"text/rfc822-headers",
		"From: a@example.com",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
}

func TestCollectRUF(t *testing.T) {
	got := collectRUF([]string{"mailto:a@example.com", "mailto:A@example.com", "http://x", "mailto:b@ex.com!50m"})
	if len(got) != 2 || got[0] != "a@example.com" || got[1] != "b@ex.com" {
		t.Fatalf("%v", got)
	}
}
