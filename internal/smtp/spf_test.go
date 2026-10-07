package smtp

import (
	"net"
	"strings"
	"testing"

	"blitiri.com.ar/go/spf"
)

func TestSPFTagPass(t *testing.T) {
	p := &spfPolicy{
		action: "tag", authservID: "mail.test",
		check: func(ip net.IP, helo, sender string) (spf.Result, error) {
			if sender != "a@example.com" {
				t.Fatalf("sender %q", sender)
			}
			return spf.Pass, nil
		},
	}
	msg := []byte("From: a@example.com\r\nSubject: x\r\n\r\nbody\r\n")
	out, res, err := p.apply("1.2.3.4", "mail.test", "a@example.com", msg)
	if err != nil || res != spf.Pass {
		t.Fatalf("res=%v err=%v", res, err)
	}
	if !strings.Contains(string(out), "spf=pass") {
		t.Fatalf("%s", out)
	}
}

func TestSPFRejectFail(t *testing.T) {
	p := &spfPolicy{
		action: "reject", authservID: "mail.test",
		check: func(net.IP, string, string) (spf.Result, error) { return spf.Fail, nil },
	}
	msg := []byte("From: a@example.com\r\n\r\nx\r\n")
	_, _, err := p.apply("1.2.3.4", "mail.test", "a@example.com", msg)
	if err == nil {
		t.Fatal("expected reject")
	}
}
