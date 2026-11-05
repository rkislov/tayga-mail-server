package smtp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"github.com/tayga/tms/internal/storage"
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
	out, domains, err := p.apply(signed)
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
	if len(domains) != 1 || domains[0] != "example.com" {
		t.Fatalf("domains %v", domains)
	}

	out2, _, err := p.apply(msg)
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
	_, _, err = p.apply(tampered)
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
	_, _, err := p.apply(msg)
	if err == nil {
		t.Fatal("expected reject without signature")
	}
}

type netDNSError struct{ msg string }

func (e *netDNSError) Error() string   { return e.msg }
func (e *netDNSError) Timeout() bool   { return false }
func (e *netDNSError) Temporary() bool { return false }

func TestManagedDKIMDomainSelection(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "key.pem")
	if err = os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); err != nil {
		t.Fatal(err)
	}
	binding := storage.DKIMBinding{Domain: "example.com", Selector: "mail", KeyFile: path, Enabled: true}
	save := func() {
		raw, _ := json.Marshal(binding)
		if err := store.PutSetting(ctx, "dkim.domain.example.com", string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	save()
	signer, err := domainDKIMSigner(ctx, store, nil, "sender@EXAMPLE.COM")
	if err != nil || signer == nil {
		t.Fatal("managed signing unavailable", err)
	}
	signed, err := signer.Sign([]byte("From: sender@example.com\r\nTo: recipient@remote.example\r\nSubject: Test\r\n\r\nBody\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	public, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	dns := "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(public)
	policy := &dkimVerifyPolicy{action: "tag", authservID: "mail.example.com", lookupTXT: func(name string) ([]string, error) {
		if name != "mail._domainkey.example.com" {
			t.Fatalf("wrong binding %s", name)
		}
		return []string{dns}, nil
	}}
	_, domains, err := policy.apply(signed)
	if err != nil || len(domains) != 1 || domains[0] != "example.com" {
		t.Fatal("signature invalid", err)
	}
	if other, err := domainDKIMSigner(ctx, store, signer, "sender@other.example"); err != nil || other != nil {
		t.Fatal("key leaked across domains")
	}
	binding.Enabled = false
	save()
	if disabled, err := domainDKIMSigner(ctx, store, signer, "sender@example.com"); err != nil || disabled != nil {
		t.Fatal("disabled binding used legacy fallback")
	}
	binding.Enabled = true
	binding.KeyFile = filepath.Join(dir, "missing.pem")
	save()
	if _, err := domainDKIMSigner(ctx, store, signer, "sender@example.com"); err == nil {
		t.Fatal("enabled broken binding must fail closed")
	}
}

func TestOutboundQueueSignsAliasDomainFromHeader(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "alias.pem")
	if err = os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); err != nil {
		t.Fatal(err)
	}
	binding := storage.DKIMBinding{Domain: "alias.example", Selector: "mail", KeyFile: path, Enabled: true}
	raw, _ := json.Marshal(binding)
	if err = store.PutSetting(ctx, "dkim.domain.alias.example", string(raw)); err != nil {
		t.Fatal(err)
	}
	capture := &aliasCaptureSender{}
	q := newOutboundQueue(store, capture, nil, nil, "mail.example.com", QueueConfig{BatchSize: 1}, nil)
	data := []byte("From: sales@alias.example\r\nTo: recipient@outside.example\r\nSubject: Alias\r\n\r\nHello\r\n")
	// SMTP clients may use their primary address for bounces while From is an alias.
	if err = q.Enqueue(ctx, "owner@example.com", []string{"recipient@outside.example"}, data, "<alias>"); err != nil {
		t.Fatal(err)
	}
	items, err := store.ClaimOutboundDue(ctx, 1)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	q.process(ctx, items[0])
	if capture.from != "owner@example.com" || !strings.Contains(string(capture.data), "DKIM-Signature:") || !strings.Contains(string(capture.data), "d=alias.example") {
		t.Fatal(capture.from, string(capture.data))
	}
}
