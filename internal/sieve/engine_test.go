package sieve_test

import (
	"strings"
	"testing"

	"github.com/tayga/tms/internal/sieve"
)

func TestCheckAndFileinto(t *testing.T) {
	script := `require ["fileinto"];
if header :contains "Subject" "spam" {
  fileinto "Junk";
}
`
	if err := sieve.CheckScript(script); err != nil {
		t.Fatal(err)
	}
	raw := []byte("From: a@example.com\r\nTo: b@example.com\r\nSubject: buy spam now\r\n\r\nbody\r\n")
	res, err := sieve.Execute(script, "a@example.com", "b@example.com", "b@example.com", raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Deliveries) != 1 || res.Deliveries[0].Mailbox != "Junk" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestReject(t *testing.T) {
	script := `require ["reject"];
reject "nope";
`
	raw := []byte("Subject: x\r\n\r\n")
	res, err := sieve.Execute(script, "a@ex.com", "b@ex.com", "b@ex.com", raw)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Rejected || !strings.Contains(res.RejectMsg, "nope") {
		t.Fatalf("expected reject: %+v", res)
	}
}
