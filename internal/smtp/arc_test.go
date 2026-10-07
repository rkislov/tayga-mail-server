package smtp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rest-mail/go-arc"
	godkim "github.com/rest-mail/go-dkim"
)

func TestARCSealAndVerify(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "arc.pem")
	der := x509.MarshalPKCS1PrivateKey(priv)
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	txt := "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(pubDER)
	resolver := godkim.TXTResolver(func(_ context.Context, name string) ([]string, error) {
		if strings.HasPrefix(name, "arc._domainkey.example.com") {
			return []string{txt}, nil
		}
		return nil, &netDNSError{msg: "nxdomain"}
	})

	p, err := newARCPolicy(true, true, "tag", true, "mail.test", "example.com", "arc", keyPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.resolver = resolver

	msg := []byte("From: a@example.com\r\nTo: b@example.com\r\nSubject: hi\r\nDate: Mon, 1 Jan 2024 00:00:00 +0000\r\nMessage-ID: <1@example.com>\r\n\r\nbody\r\n")
	// Pre-inject auth results as SPF/DKIM would.
	msg = append([]byte("Authentication-Results: mail.test; spf=pass smtp.mailfrom=a@example.com\r\n"), msg...)

	out, err := p.apply(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "ARC-Seal:") || !strings.Contains(string(out), "ARC-Message-Signature:") {
		t.Fatalf("missing ARC headers: %s", out)
	}
	if !strings.Contains(string(out), "Authentication-Results: mail.test; arc=") {
		t.Fatalf("missing arc AR: %s", out)
	}

	cv, reason := arc.Verify(context.Background(), out, resolver)
	if cv != "pass" {
		t.Fatalf("verify cv=%s reason=%s", cv, reason)
	}
}

func TestARCVerifyNone(t *testing.T) {
	p := &arcPolicy{verify: true, action: "tag", authservID: "mail.test"}
	msg := []byte("From: a@example.com\r\nSubject: x\r\n\r\ny\r\n")
	out, err := p.apply(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "arc=none") {
		t.Fatalf("%s", out)
	}
}

func TestCollectAuthResultsForARC(t *testing.T) {
	msg := []byte("Authentication-Results: mail.test; spf=pass\r\nAuthentication-Results: mail.test; dkim=pass header.d=ex.com\r\nFrom: a@ex.com\r\n\r\nx\r\n")
	got := collectAuthResultsForARC(msg, "mail.test")
	if !strings.Contains(got, "spf=pass") || !strings.Contains(got, "dkim=pass") {
		t.Fatal(got)
	}
}
