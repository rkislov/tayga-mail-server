package smtp

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/tayga/tms/internal/config"
)

type dkimSigner struct {
	domain   string
	selector string
	key      crypto.Signer
}

func newDKIMSigner(cfg config.SMTPDKIMConfig) (*dkimSigner, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	raw, err := os.ReadFile(cfg.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("dkim key: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("dkim key: no PEM block")
	}
	var key crypto.Signer
	switch block.Type {
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("dkim pkcs8: %w", err)
		}
		switch t := k.(type) {
		case *rsa.PrivateKey:
			key = t
		case ed25519.PrivateKey:
			key = t
		default:
			return nil, fmt.Errorf("dkim: unsupported key type %T", k)
		}
	case "RSA PRIVATE KEY":
		k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("dkim pkcs1: %w", err)
		}
		key = k
	default:
		return nil, fmt.Errorf("dkim key: unsupported PEM type %q", block.Type)
	}
	return &dkimSigner{
		domain:   strings.ToLower(cfg.Domain),
		selector: cfg.Selector,
		key:      key,
	}, nil
}

func (s *dkimSigner) Sign(msg []byte) ([]byte, error) {
	if s == nil {
		return msg, nil
	}
	opts := &dkim.SignOptions{
		Domain:   s.domain,
		Selector: s.selector,
		Signer:   s.key,
		HeaderKeys: []string{
			"From", "To", "Subject", "Date", "Message-ID", "MIME-Version", "Content-Type",
		},
	}
	var buf bytes.Buffer
	if err := dkim.Sign(&buf, bytes.NewReader(msg), opts); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
