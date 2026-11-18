// Package vks provides authenticated conferencing infrastructure for TMS.
package vks

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ICEConfig is server-side configuration. Secret is never returned to clients.
type ICEConfig struct {
	STUNURLs []string      `json:"stun_urls" yaml:"stun_urls"`
	TURNURLs []string      `json:"turn_urls" yaml:"turn_urls"`
	Secret   string        `json:"-" yaml:"secret"`
	TTL      time.Duration `json:"-" yaml:"ttl"`
}
type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}
type ICECredentials struct {
	ICEServers []ICEServer `json:"ice_servers"`
	ExpiresAt  int64       `json:"expires_at"`
}

func ValidateURLs(values []string, turn bool) error {
	if len(values) > 8 {
		return errors.New("at most eight ICE URLs per kind")
	}
	for _, raw := range values {
		u, err := url.Parse(raw)
		if err != nil || u.User != nil || u.Fragment != "" || u.Host != "" || u.Opaque == "" {
			return errors.New("invalid ICE URL")
		}
		if turn && u.Scheme != "turn" && u.Scheme != "turns" || !turn && u.Scheme != "stun" && u.Scheme != "stuns" {
			return errors.New("invalid ICE URL scheme")
		}
		host := u.Opaque
		if strings.ContainsAny(host, " /@\r\n\t#") {
			return errors.New("invalid ICE host")
		}
		if _, err := url.Parse("https://" + host); err != nil {
			return errors.New("invalid ICE host")
		}
		if u.RawQuery != "" && (u.Scheme == "stun" || u.Scheme == "stuns" || u.RawQuery != "transport=udp" && u.RawQuery != "transport=tcp") {
			return errors.New("invalid ICE transport")
		}
	}
	return nil
}

// Credentials uses coturn's TURN REST authentication. Identity includes scope
// without exposing account identifiers. Every request gets a bounded lifetime.
func (c ICEConfig) Credentials(now time.Time, tenant, domain, user string) (ICECredentials, error) {
	out := ICECredentials{ICEServers: []ICEServer{}}
	if tenant == "" || domain == "" || user == "" {
		return out, errors.New("ICE requires authenticated domain identity")
	}
	if err := ValidateURLs(c.STUNURLs, false); err != nil {
		return out, err
	}
	if err := ValidateURLs(c.TURNURLs, true); err != nil {
		return out, err
	}
	if len(c.STUNURLs) > 0 {
		out.ICEServers = append(out.ICEServers, ICEServer{URLs: append([]string(nil), c.STUNURLs...)})
	}
	if len(c.TURNURLs) == 0 {
		return out, nil
	}
	if len(c.Secret) < 32 {
		return out, errors.New("TURN shared secret must be at least 32 bytes")
	}
	ttl := c.TTL
	if ttl == 0 {
		ttl = time.Hour
	}
	if ttl < time.Minute || ttl > 24*time.Hour {
		return out, errors.New("TURN lifetime must be between one minute and 24 hours")
	}
	out.ExpiresAt = now.Add(ttl).Unix()
	identity := sha256.Sum256([]byte(tenant + "\x00" + domain + "\x00" + user))
	username := fmt.Sprintf("%d:%s", out.ExpiresAt, hex.EncodeToString(identity[:16]))
	mac := hmac.New(sha1.New, []byte(c.Secret)) // coturn TURN REST mandates HMAC-SHA1.
	_, _ = mac.Write([]byte(username))
	out.ICEServers = append(out.ICEServers, ICEServer{URLs: append([]string(nil), c.TURNURLs...), Username: username, Credential: base64.StdEncoding.EncodeToString(mac.Sum(nil))})
	return out, nil
}
