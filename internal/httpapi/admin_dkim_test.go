package httpapi

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
)

func TestGenerateDKIMMatchingDNSKey(t *testing.T) {
	result, err := generateDKIM("example.com", "tayga")
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode([]byte(result["private_key"]))
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(result["dns_value"], "v=DKIM1; k=rsa; p="))
	if err != nil {
		t.Fatal(err)
	}
	public, err := x509.ParsePKIXPublicKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	if public.(*rsa.PublicKey).N.Cmp(key.N) != 0 || key.N.BitLen() != 2048 || result["dns_name"] != "tayga._domainkey.example.com" {
		t.Fatal("DNS key does not match private key")
	}
}
