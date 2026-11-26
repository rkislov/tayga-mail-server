package vks

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
	"os"
	"strings"
	"time"
)

type Meeting struct {
	Provider     Provider  `json:"provider,omitempty"`
	KonturURL    string    `json:"kontur_url,omitempty"`
	ChannelID    string    `json:"channel_id,omitempty"`
	Room         string    `json:"room"`
	Title        string    `json:"title"`
	TenantID     string    `json:"tenant_id"`
	DomainID     string    `json:"domain_id"`
	OwnerID      string    `json:"owner_id"`
	Participants []string  `json:"participants"`
	CreatedAt    time.Time `json:"created_at"`
}
type Meetings struct {
	Store  storage.Driver
	Policy Effective
}

func (v Meeting) Backend() Provider {
	if v.Provider == "" {
		return ProviderJitsi
	}
	return v.Provider
}

func (m Meetings) Get(ctx context.Context, room string) (Meeting, error) {
	var v Meeting
	if len(room) != 36 || !strings.HasPrefix(room, "tms-") {
		return v, storage.ErrNotFound
	}
	raw, ok, err := m.Store.GetSetting(ctx, "vks.meeting:"+room)
	if err != nil {
		return v, err
	}
	if !ok {
		return v, storage.ErrNotFound
	}
	err = json.Unmarshal([]byte(raw), &v)
	return v, err
}
func (v Meeting) Allows(u *storage.User) bool {
	if v.ChannelID != "" || u == nil || !u.Enabled || u.TenantID != v.TenantID || u.DomainID != v.DomainID {
		return false
	}
	if u.ID == v.OwnerID {
		return true
	}
	for _, id := range v.Participants {
		if id == u.ID {
			return true
		}
	}
	return false
}
func (m Meetings) Create(ctx context.Context, u *storage.User, title string, participants []string) (Meeting, error) {
	var out Meeting
	if u == nil || !u.Enabled || u.TenantID == "" || u.DomainID == "" {
		return out, errors.New("authenticated domain identity required")
	}
	title = strings.TrimSpace(title)
	if len(title) == 0 || len(title) > 240 || len(participants) > 100 {
		return out, errors.New("invalid meeting title or participant count")
	}
	ids := []string{}
	seen := map[string]bool{u.ID: true}
	for _, id := range participants {
		if seen[id] {
			continue
		}
		var p *storage.User
		var err error
		if strings.Contains(id, "@") {
			p, err = m.Store.GetUserByEmail(ctx, strings.ToLower(id))
		} else {
			p, err = m.Store.GetUserByID(ctx, id)
		}
		if err != nil || !p.Enabled || p.TenantID != u.TenantID || p.DomainID != u.DomainID {
			return out, errors.New("participant must be an active user of this domain")
		}
		seen[p.ID] = true
		ids = append(ids, p.ID)
	}
	return m.save(ctx, Meeting{Title: title, TenantID: u.TenantID, DomainID: u.DomainID, OwnerID: u.ID, Participants: ids})
}

func (m Meetings) save(ctx context.Context, meeting Meeting) (Meeting, error) {
	meeting.Provider = m.Policy.Provider
	if meeting.Provider == ProviderKontur {
		space, err := KonturSpace(m.Policy.KonturURL)
		if err != nil {
			return Meeting{}, err
		}
		meeting.KonturURL = space
	}
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return Meeting{}, err
	}
	meeting.Room = "tms-" + hex.EncodeToString(data)
	meeting.CreatedAt = time.Now().UTC()
	raw, err := json.Marshal(meeting)
	if err != nil {
		return Meeting{}, err
	}
	if err = m.Store.PutSetting(ctx, "vks.meeting:"+meeting.Room, string(raw)); err != nil {
		return Meeting{}, err
	}
	return meeting, nil
}

func (m Meetings) CreateForChannel(ctx context.Context, u *storage.User, channelID string) (Meeting, error) {
	c, err := chat.New(m.Store).Channel(ctx, u, channelID)
	if err != nil {
		return Meeting{}, err
	}
	title := c.DisplayName
	if title == "" {
		title = "Звонок в личном чате"
	}
	return m.save(ctx, Meeting{ChannelID: c.ID, Title: title, TenantID: u.TenantID, DomainID: u.DomainID, OwnerID: u.ID, Participants: []string{}})
}

func (m Meetings) CanJoin(ctx context.Context, meeting Meeting, u *storage.User) bool {
	if meeting.ChannelID == "" {
		return meeting.Allows(u)
	}
	if u == nil || !u.Enabled || u.TenantID != meeting.TenantID || time.Since(meeting.CreatedAt) >= 12*time.Hour {
		return false
	}
	_, err := chat.New(m.Store).Channel(ctx, u, meeting.ChannelID)
	return err == nil
}

func LoadSigningKey(path string) (*rsa.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b, _ := pem.Decode(raw)
	if b == nil {
		return nil, errors.New("invalid VKS signing key")
	}
	var key *rsa.PrivateKey
	if b.Type == "RSA PRIVATE KEY" {
		key, err = x509.ParsePKCS1PrivateKey(b.Bytes)
	} else {
		var parsed any
		parsed, err = x509.ParsePKCS8PrivateKey(b.Bytes)
		if err == nil {
			var ok bool
			key, ok = parsed.(*rsa.PrivateKey)
			if !ok {
				return nil, errors.New("VKS requires RSA signing key")
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if key.N.BitLen() < 2048 {
		return nil, errors.New("VKS RSA key must be at least 2048 bits")
	}
	return key, key.Validate()
}
func PublicKeyPEM(key *rsa.PrivateKey) ([]byte, error) {
	raw, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: raw}), nil
}
func KeyFilename(kid string) string {
	sum := sha256.Sum256([]byte(kid))
	return hex.EncodeToString(sum[:]) + ".pem"
}

// JoinToken is bound to one exact room and participant; callers must enforce
// current domain policy before issuance. No wildcard room or guest tokens.
func JoinToken(key *rsa.PrivateKey, kid, subject string, meeting Meeting, u *storage.User, now time.Time) (string, error) {
	if !meeting.Allows(u) {
		return "", errors.New("meeting access denied")
	}
	return signJoinToken(key, kid, subject, meeting, u, now)
}

func (m Meetings) JoinToken(ctx context.Context, key *rsa.PrivateKey, kid, subject string, meeting Meeting, u *storage.User, now time.Time) (string, error) {
	if !m.CanJoin(ctx, meeting, u) {
		return "", errors.New("meeting access denied")
	}
	return signJoinToken(key, kid, subject, meeting, u, now)
}

func signJoinToken(key *rsa.PrivateKey, kid, subject string, meeting Meeting, u *storage.User, now time.Time) (string, error) {
	if meeting.Backend() != ProviderJitsi || kid == "" || subject == "" {
		return "", errors.New("invalid conference signing configuration")
	}
	affiliation := "member"
	if u.ID == meeting.OwnerID {
		affiliation = "owner"
	}
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid})
	claims, _ := json.Marshal(map[string]any{"iss": "tms", "aud": "tms", "sub": subject, "room": meeting.Room, "iat": now.Unix(), "nbf": now.Add(-30 * time.Second).Unix(), "exp": now.Add(30 * time.Minute).Unix(), "context": map[string]any{"user": map[string]any{"id": u.ID, "name": u.DisplayName, "email": u.Email, "affiliation": affiliation, "moderator": u.ID == meeting.OwnerID}}})
	signing := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	hash := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// VerifyJoinToken validates the exact JWT contract issued by TMS before a focus
// allocation. The caller must additionally recheck the stored meeting and user.
func VerifyJoinToken(key *rsa.PublicKey, kid, subject, token string, now time.Time) (string, string, error) {
	denied := errors.New("invalid conference token")
	if len(token) > 16384 {
		return "", "", denied
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", "", denied
	}
	decode := func(part string, out any) error {
		raw, e := base64.RawURLEncoding.DecodeString(part)
		if e != nil {
			return e
		}
		return json.Unmarshal(raw, out)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if decode(parts[0], &header) != nil || header.Alg != "RS256" || header.Kid != kid {
		return "", "", denied
	}
	sig, e := base64.RawURLEncoding.DecodeString(parts[2])
	if e != nil {
		return "", "", denied
	}
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], sig) != nil {
		return "", "", denied
	}
	var claims struct {
		Issuer    string `json:"iss"`
		Audience  string `json:"aud"`
		Subject   string `json:"sub"`
		Room      string `json:"room"`
		Expires   int64  `json:"exp"`
		NotBefore int64  `json:"nbf"`
		Context   struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"context"`
	}
	if decode(parts[1], &claims) != nil || claims.Issuer != "tms" || claims.Audience != "tms" || claims.Subject != subject || claims.Expires <= now.Unix() || claims.NotBefore > now.Unix() || claims.Room == "" || claims.Room == "*" || claims.Context.User.ID == "" {
		return "", "", denied
	}
	return claims.Room, claims.Context.User.ID, nil
}
