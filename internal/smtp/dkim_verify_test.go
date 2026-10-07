package smtp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
)

func TestDKIMVerifyPassAndNone(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "dkim.pem")
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(priv)
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	sig, err := newDKIMSigner(config.SMTPDKIMConfig{
		Enabled: true, Domain: "example.com", Selector: "tayga", PrivateKeyFile: keyPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("From: a@example.com\r\nTo: b@example.com\r\nSubject: hi\r\n\r\nbody\r\n")
	signed, err := sig.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	txt := "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(pubDER)
	lookup := func(domain string) ([]string, error) {
		if domain == "tayga._domainkey.example.com" {
			return []string{txt}, nil
		}
		return nil, &netDNSError{msg: "nxdomain"}
	}

	p := &dkimVerifyPolicy{
		action: "tag", authservID: "mail.test", lookupTXT: lookup,
	}
	out, err := p.apply(signed)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "Authentication-Results: mail.test; dkim=pass") {
		t.Fatalf("want pass: %s", s)
	}
	if !strings.Contains(s, "header.d=example.com") {
		t.Fatalf("want domain: %s", s)
	}

	out2, err := p.apply(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out2), "Authentication-Results: mail.test; dkim=none") {
		t.Fatalf("%s", out2)
	}
}

func TestDKIMVerifyRejectFail(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "dkim.pem")
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(priv)
	_ = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}), 0o600)
	sig, err := newDKIMSigner(config.SMTPDKIMConfig{
		Enabled: true, Domain: "example.com", Selector: "tayga", PrivateKeyFile: keyPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("From: a@example.com\r\nTo: b@example.com\r\nSubject: hi\r\n\r\nbody\r\n")
	signed, err := sig.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt body after signing.
	tampered := append([]byte{}, signed...)
	tampered = append(tampered, []byte("X")...)

	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, _ := x509.MarshalPKIXPublicKey(&other.PublicKey)
	txt := "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(pubDER)
	lookup := func(domain string) ([]string, error) {
		if strings.HasPrefix(domain, "tayga._domainkey.") {
			return []string{txt}, nil
		}
		return nil, &netDNSError{msg: "nxdomain"}
	}

	p := &dkimVerifyPolicy{action: "reject", authservID: "mail.test", lookupTXT: lookup}
	_, err = p.apply(tampered)
	if err == nil {
		t.Fatal("expected reject")
	}
}

func TestDKIMVerifyRequireSignature(t *testing.T) {
	p := &dkimVerifyPolicy{
		action: "reject", requireSignature: true, authservID: "mail.test",
		lookupTXT: func(string) ([]string, error) { return nil, &netDNSError{msg: "nxdomain"} },
	}
	msg := []byte("From: a@example.com\r\nSubject: x\r\n\r\ny\r\n")
	_, err := p.apply(msg)
	if err == nil {
		t.Fatal("expected reject without signature")
	}
}

type netDNSError struct{ msg string }

func (e *netDNSError) Error() string   { return e.msg }
func (e *netDNSError) Timeout() bool   { return false }
func (e *netDNSError) Temporary() bool { return false }
