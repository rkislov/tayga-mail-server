package smtp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
)

func TestDKIMSignAddsHeader(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "dkim.pem")
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(priv)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(keyPath, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	sig, err := newDKIMSigner(config.SMTPDKIMConfig{
		Enabled: true, Domain: "example.com", Selector: "tayga", PrivateKeyFile: keyPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("From: a@example.com\r\nTo: b@example.com\r\nSubject: hi\r\n\r\nbody\r\n")
	out, err := sig.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "DKIM-Signature:") {
		t.Fatalf("missing DKIM-Signature: %s", out)
	}
}
