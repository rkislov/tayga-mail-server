package vks

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCoturnRESTCredentials(t *testing.T) {
	now := time.Unix(1800000000, 0)
	c := ICEConfig{STUNURLs: []string{"stun:meet.example.test:3478"}, TURNURLs: []string{"turn:meet.example.test:3478?transport=udp", "turns:meet.example.test:5349?transport=tcp"}, Secret: strings.Repeat("s", 40), TTL: time.Hour}
	out, err := c.Credentials(now, "tenant", "domain", "user")
	if err != nil {
		t.Fatal(err)
	}
	relay := out.ICEServers[1]
	if out.ExpiresAt != 1800003600 || !strings.HasPrefix(relay.Username, "1800003600:") {
		t.Fatal(out)
	}
	mac := hmac.New(sha1.New, []byte(c.Secret))
	mac.Write([]byte(relay.Username))
	if relay.Credential != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
		t.Fatal("coturn credential mismatch")
	}
	other, _ := c.Credentials(now, "tenant", "sibling", "user")
	if other.ICEServers[1].Username == relay.Username {
		t.Fatal("domain identity collision")
	}
	data, _ := json.Marshal(out)
	if strings.Contains(string(data), c.Secret) || strings.Contains(string(data), "tenant") {
		t.Fatal("secret or identity disclosure")
	}
	c.TTL = 25 * time.Hour
	if _, err = c.Credentials(now, "t", "d", "u"); err == nil {
		t.Fatal("unbounded lifetime")
	}
	c.TTL = time.Hour
	c.Secret = "weak"
	if _, err = c.Credentials(now, "t", "d", "u"); err == nil {
		t.Fatal("weak secret accepted")
	}
}
func TestRejectMalformedICEURLs(t *testing.T) {
	for _, raw := range []string{"https://host", "turn://user:pass@host", "turn:host?transport=bad", "turn:host/path", "turn:host\n", "turn:host#fragment"} {
		if ValidateURLs([]string{raw}, true) == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
