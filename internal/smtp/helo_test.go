package smtp

import (
	"strings"
	"testing"
)

func TestEvaluateHelo(t *testing.T) {
	cases := []struct {
		helo string
		fqdn bool
		want string
	}{
		{"mail.example.com", true, "pass"},
		{"mail", true, "fail"},
		{"mail", false, "pass"},
		{"localhost", true, "fail"},
		{"1.2.3.4", true, "fail"},
		{"[1.2.3.4]", true, "pass"},
		{"", true, "fail"},
		{"bad..host", true, "fail"},
	}
	for _, tc := range cases {
		got, _ := evaluateHelo(tc.helo, tc.fqdn)
		if got != tc.want {
			t.Fatalf("%q fqdn=%v: got %s want %s", tc.helo, tc.fqdn, got, tc.want)
		}
	}
}

func TestHeloApplyReject(t *testing.T) {
	p := &heloPolicy{action: "reject", requireFQDN: true, authservID: "mail.test"}
	_, err := p.apply("localhost", []byte("From: a@b\r\n\r\nx\r\n"))
	if err == nil {
		t.Fatal("expected reject")
	}
	out, err := p.apply("mx.example.com", []byte("From: a@b\r\n\r\nx\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "helo=pass") {
		t.Fatalf("%s", out)
	}
}
