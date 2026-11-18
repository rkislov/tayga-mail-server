package vks

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestRoomTokenAndDomainIsolation(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	u := &storage.User{ID: "owner", TenantID: "t", DomainID: "d", Enabled: true}
	m := Meeting{Room: "tms-" + strings.Repeat("a", 32), TenantID: "t", DomainID: "d", OwnerID: "owner", Participants: []string{"member"}}
	now := time.Unix(1800000000, 0)
	token, err := JoinToken(key, "key1", "meet.example.test", m, u, now)
	if err != nil {
		t.Fatal(err)
	}
	room, uid, e := VerifyJoinToken(&key.PublicKey, "key1", "meet.example.test", token, now)
	if e != nil || room != m.Room || uid != u.ID {
		t.Fatal("verification failed", e)
	}
	if _, _, e = VerifyJoinToken(&key.PublicKey, "key1", "meet.example.test", token, now.Add(time.Hour)); e == nil {
		t.Fatal("expired token accepted")
	}
	if _, _, e = VerifyJoinToken(&key.PublicKey, "key1", "other.test", token, now); e == nil {
		t.Fatal("foreign audience accepted")
	}
	parts := strings.Split(token, ".")
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hash[:], sig) != nil {
		t.Fatal("invalid RSA signature")
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	json.Unmarshal(raw, &claims)
	if claims["room"] != m.Room || claims["exp"].(float64) != float64(now.Add(30*time.Minute).Unix()) || claims["sub"] != "meet.example.test" {
		t.Fatal(claims)
	}
	u.DomainID = "sibling"
	if _, err = JoinToken(key, "key1", "meet.example.test", m, u, now); err == nil {
		t.Fatal("foreign domain allowed")
	}
	u.DomainID = "d"
	u.ID = "outsider"
	if _, err = JoinToken(key, "key1", "meet.example.test", m, u, now); err == nil {
		t.Fatal("uninvited user allowed")
	}
	u.ID = "member"
	if _, err = JoinToken(key, "key1", "meet.example.test", m, u, now); err != nil {
		t.Fatal(err)
	}
	u.Enabled = false
	if m.Allows(u) {
		t.Fatal("disabled participant allowed")
	}
}
