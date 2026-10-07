package smtp

import (
	"net"
	"strings"
	"testing"
)

func TestIPRevPass(t *testing.T) {
	p := &iprevPolicy{
		action: "tag", authservID: "mail.test",
		lookupAddr: func(ip string) ([]string, error) {
			return []string{"mail.example.com."}, nil
		},
		lookupIP: func(host string) ([]net.IP, error) {
			if host != "mail.example.com" {
				t.Fatalf("host %q", host)
			}
			return []net.IP{net.ParseIP("203.0.113.10")}, nil
		},
	}
	out, err := p.apply("203.0.113.10", []byte("From: a@b\r\n\r\nx\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "iprev=pass") || !strings.Contains(string(out), "policy.ptr=mail.example.com") {
		t.Fatalf("%s", out)
	}
}

func TestIPRevFailReject(t *testing.T) {
	p := &iprevPolicy{
		action: "reject", authservID: "mail.test",
		lookupAddr: func(string) ([]string, error) { return nil, &net.DNSError{IsNotFound: true, Err: "no such host"} },
	}
	_, err := p.apply("203.0.113.10", []byte("From: a@b\r\n\r\nx\r\n"))
	if err == nil {
		t.Fatal("expected reject")
	}
}

func TestIPRevMismatch(t *testing.T) {
	p := &iprevPolicy{
		action: "tag", authservID: "mail.test",
		lookupAddr: func(string) ([]string, error) { return []string{"mail.example.com."}, nil },
		lookupIP:   func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("198.51.100.1")}, nil },
	}
	out, err := p.apply("203.0.113.10", []byte("From: a@b\r\n\r\nx\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "iprev=fail") {
		t.Fatalf("%s", out)
	}
}
