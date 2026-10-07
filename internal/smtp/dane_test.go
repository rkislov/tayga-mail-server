package smtp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

func TestMatchTLSAUsage3SHA256(t *testing.T) {
	cert := testCert(t)
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	recs := []tlsaRecord{{
		Usage: 3, Selector: 1, MatchingType: 1,
		Certificate: sum[:],
	}}
	cs := tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}
	if err := matchTLSA(cs, recs); err != nil {
		t.Fatal(err)
	}
	recs[0].Certificate = make([]byte, 32) // wrong hash
	if err := matchTLSA(cs, recs); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestMatchTLSAExactFullCert(t *testing.T) {
	cert := testCert(t)
	recs := []tlsaRecord{{
		Usage: 3, Selector: 0, MatchingType: 0,
		Certificate: append([]byte(nil), cert.Raw...),
	}}
	cs := tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}
	if err := matchTLSA(cs, recs); err != nil {
		t.Fatal(err)
	}
}

func TestDANETlsConfigNoTLSA(t *testing.T) {
	r := newDANEResolver(time.Second, true, nil)
	r.exchange = func(string) ([]tlsaRecord, error) { return nil, nil }
	cfg, req, err := r.tlsConfigFor("mx.example.com")
	if err != nil || req || cfg.ServerName != "mx.example.com" {
		t.Fatalf("cfg=%v req=%v err=%v", cfg, req, err)
	}
}

func TestDANETlsConfigWithTLSA(t *testing.T) {
	cert := testCert(t)
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	r := newDANEResolver(time.Second, false, nil)
	r.exchange = func(string) ([]tlsaRecord, error) {
		return []tlsaRecord{{Usage: 3, Selector: 1, MatchingType: 1, Certificate: sum[:]}}, nil
	}
	cfg, req, err := r.tlsConfigFor("mx.example.com")
	if err != nil || !req || !cfg.InsecureSkipVerify || cfg.VerifyConnection == nil {
		t.Fatalf("cfg=%+v req=%v err=%v", cfg, req, err)
	}
	cs := tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}
	if err := cfg.VerifyConnection(cs); err != nil {
		t.Fatal(err)
	}
}

func testCert(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "mx.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}
