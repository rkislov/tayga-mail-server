package httpapi

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Public keys from HTTPS discovery authenticate WOPI callbacks. Private keys
// stay at Collabora. A host without proof keys still requires scoped tokens.
type officeProofKeys struct {
	Modulus     string `xml:"modulus,attr"`
	Exponent    string `xml:"exponent,attr"`
	OldModulus  string `xml:"oldmodulus,attr"`
	OldExponent string `xml:"oldexponent,attr"`
}

func (p officeProofKeys) key(old bool) *rsa.PublicKey {
	m, e := p.Modulus, p.Exponent
	if old {
		m, e = p.OldModulus, p.OldExponent
	}
	n, err := base64.StdEncoding.DecodeString(m)
	if err != nil || len(n) < 256 || len(n) > 1024 {
		return nil
	}
	exp, err := base64.StdEncoding.DecodeString(e)
	if err != nil || len(exp) == 0 || len(exp) > 4 {
		return nil
	}
	v := new(big.Int).SetBytes(exp).Int64()
	if v < 3 || v > 2147483647 || v%2 == 0 {
		return nil
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(v)}
}

func officeProofData(token, address string, ticks int64) []byte {
	var b bytes.Buffer
	for _, value := range []string{token, strings.ToUpper(address)} {
		_ = binary.Write(&b, binary.BigEndian, uint32(len(value)))
		b.WriteString(value)
	}
	_ = binary.Write(&b, binary.BigEndian, uint32(8))
	_ = binary.Write(&b, binary.BigEndian, ticks)
	return b.Bytes()
}

func (p officeProofKeys) valid(r *http.Request, token, address string, now time.Time) bool {
	ticks, err := strconv.ParseInt(r.Header.Get("X-WOPI-TimeStamp"), 10, 64)
	// WOPI timestamps are 100 ns ticks since 0001-01-01, big endian.
	current := now.Unix()*10000000 + 621355968000000000 + int64(now.Nanosecond()/100)
	window := int64(20 * time.Minute / 100)
	if err != nil || ticks < current-window || ticks > current+window {
		return false
	}
	hash := sha256.Sum256(officeProofData(token, address, ticks))
	verify := func(key *rsa.PublicKey, header string) bool {
		if key == nil {
			return false
		}
		sig, err := base64.StdEncoding.DecodeString(r.Header.Get(header))
		return err == nil && rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], sig) == nil
	}
	return verify(p.key(false), "X-WOPI-Proof") || verify(p.key(false), "X-WOPI-ProofOld") || verify(p.key(true), "X-WOPI-Proof")
}
