package smtp

import (
	"context"
	"strings"
	"testing"

	"blitiri.com.ar/go/spf"
	"github.com/emersion/go-msgauth/dmarc"
)

func TestDMARCPassDKIMAligned(t *testing.T) {
	p := &dmarcPolicy{
		action: "tag", authservID: "mail.test",
		lookup: func(domain string) (*dmarc.Record, error) {
			if domain != "example.com" {
				t.Fatalf("domain %q", domain)
			}
			return &dmarc.Record{Policy: dmarc.PolicyReject, DKIMAlignment: dmarc.AlignmentRelaxed, SPFAlignment: dmarc.AlignmentRelaxed}, nil
		},
	}
	msg := []byte("From: User <a@example.com>\r\nSubject: hi\r\n\r\nbody\r\n")
	out, err := p.apply(context.Background(), msg, "other@evil.test", "1.2.3.4", spf.Fail, []string{"example.com"}, "none")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "dmarc=pass") {
		t.Fatalf("%s", out)
	}
}

func TestDMARCFollowReject(t *testing.T) {
	p := &dmarcPolicy{
		action: "follow", authservID: "mail.test",
		lookup: func(string) (*dmarc.Record, error) {
			return &dmarc.Record{Policy: dmarc.PolicyReject, DKIMAlignment: dmarc.AlignmentRelaxed, SPFAlignment: dmarc.AlignmentRelaxed}, nil
		},
	}
	msg := []byte("From: a@example.com\r\n\r\nx\r\n")
	_, err := p.apply(context.Background(), msg, "a@example.com", "1.2.3.4", spf.Fail, nil, "none")
	if err == nil {
		t.Fatal("expected reject")
	}
}

func TestDMARCNoPolicy(t *testing.T) {
	p := &dmarcPolicy{
		action: "follow", authservID: "mail.test",
		lookup: func(string) (*dmarc.Record, error) { return nil, dmarc.ErrNoPolicy },
	}
	msg := []byte("From: a@example.com\r\n\r\nx\r\n")
	out, err := p.apply(context.Background(), msg, "a@example.com", "1.2.3.4", spf.Fail, nil, "none")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "dmarc=none") {
		t.Fatalf("%s", out)
	}
}

func TestDMARCARCTrustSoftensReject(t *testing.T) {
	p := &dmarcPolicy{
		action: "follow", arcTrust: true, authservID: "mail.test",
		lookup: func(string) (*dmarc.Record, error) {
			return &dmarc.Record{Policy: dmarc.PolicyReject, DKIMAlignment: dmarc.AlignmentRelaxed, SPFAlignment: dmarc.AlignmentRelaxed}, nil
		},
	}
	msg := []byte("From: a@example.com\r\n\r\nx\r\n")
	out, err := p.apply(context.Background(), msg, "a@example.com", "1.2.3.4", spf.Fail, nil, "pass")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "dmarc=fail") {
		t.Fatalf("expected dmarc=fail: %s", s)
	}
	if !strings.Contains(s, "reason=\"arc-pass\"") {
		t.Fatalf("expected arc-pass reason: %s", s)
	}
}

func TestAlignedRelaxed(t *testing.T) {
	if !aligned("mail.example.com", "example.com", dmarc.AlignmentRelaxed) {
		t.Fatal("expected relaxed align")
	}
	if aligned("mail.example.com", "example.com", dmarc.AlignmentStrict) {
		t.Fatal("strict should fail")
	}
}
