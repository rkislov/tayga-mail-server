// SPDX-License-Identifier: Apache-2.0
// Package mmfederation implements opt-in, domain-scoped shared-channel links.
package mmfederation

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

var ErrInvalid = errors.New("invalid federation request")
var ErrForbidden = errors.New("federation access denied")

type Peer struct {
	ID            string `json:"id"`
	TenantID      string `json:"tenant_id"`
	DomainID      string `json:"domain_id"`
	OwnerID       string `json:"owner_id"`
	Name          string `json:"name"`
	Mode          string `json:"mode"`
	SiteURL       string `json:"site_url"`
	Enabled       bool   `json:"enabled"`
	Confirmed     bool   `json:"confirmed"`
	CreatedAt     int64  `json:"create_at"`
	LastSuccessAt int64  `json:"last_success_at"`
	LastError     string `json:"last_error"`
}
type Credentials struct {
	Token          string
	RemoteToken    string
	AccessToken    string
	BootstrapToken string
}
type Link struct {
	LocalReadOnly   bool   `json:"local_readonly"`
	LeaseToken      string `json:"-"`
	ID              string `json:"id"`
	PeerID          string `json:"peer_id"`
	ChannelID       string `json:"channel_id"`
	RemoteChannelID string `json:"remote_channel_id"`
	Enabled         bool   `json:"enabled"`
	ReadOnly        bool   `json:"readonly"`
	CreatedAt       int64  `json:"create_at"`
	LocalCursorAt   int64  `json:"-"`
	LocalCursorID   string `json:"-"`
	RemoteCursorAt  int64  `json:"-"`
	RemoteCursorID  string `json:"-"`
}
type Service struct {
	ScopeAllowed func(context.Context, string, string) bool
	Store        storage.Driver
	Chat         *chat.Service
	PublicURL    string
	HTTP         *http.Client
	Log          *slog.Logger
	cipher       cipher.AEAD
	db           *sql.DB
	postgres     bool
}

func New(store storage.Driver, c *chat.Service, aead cipher.AEAD, publicURL string, client *http.Client, log *slog.Logger) *Service {
	s := &Service{Store: store, Chat: c, cipher: aead, PublicURL: strings.TrimRight(publicURL, "/"), HTTP: client, Log: log, db: store.DB()}
	if d, ok := store.(interface{ DialectName() string }); ok {
		s.postgres = d.DialectName() == "postgres"
	}
	if s.Log == nil {
		s.Log = slog.Default()
	}
	return s
}
func (s *Service) q(query string) string {
	if !s.postgres {
		return query
	}
	n := 0
	var b strings.Builder
	for _, r := range query {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func siteURL(raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(raw, "\r\n\x00") {
		return "", ErrInvalid
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}
func peerAAD(p Peer) []byte {
	return []byte(p.ID + ":" + p.TenantID + ":" + p.DomainID + ":" + p.Mode + ":" + p.SiteURL + ":" + p.OwnerID)
}

func (s *Service) seal(p Peer, c Credentials) (string, error) {
	if s.cipher == nil {
		return "", ErrForbidden
	}
	raw, e := json.Marshal(c)
	if e != nil {
		return "", e
	}
	nonce := make([]byte, s.cipher.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return "", e
	}
	return base64.StdEncoding.EncodeToString(s.cipher.Seal(nonce, nonce, raw, peerAAD(p))), nil
}
func (s *Service) credentials(ctx context.Context, p Peer) (Credentials, error) {
	var c Credentials
	var secret string
	e := s.db.QueryRowContext(ctx, s.q(`SELECT secret FROM chat_federation_peers WHERE id=? AND tenant_id=? AND domain_id=? AND enabled=1`), p.ID, p.TenantID, p.DomainID).Scan(&secret)
	if e != nil {
		return c, e
	}
	data, e := base64.StdEncoding.DecodeString(secret)
	if e != nil || s.cipher == nil || len(data) < s.cipher.NonceSize() {
		return c, ErrForbidden
	}
	n := s.cipher.NonceSize()
	plain, e := s.cipher.Open(nil, data[:n], data[n:], peerAAD(p))
	if e != nil {
		return c, ErrForbidden
	}
	e = json.Unmarshal(plain, &c)
	return c, e
}

const peerColumns = "id,tenant_id,domain_id,owner_id,name,mode,site_url,enabled,confirmed,created_at,last_success_at,last_error"

func scanPeer(row interface{ Scan(...any) error }) (Peer, error) {
	var p Peer
	var enabled, confirmed int
	e := row.Scan(&p.ID, &p.TenantID, &p.DomainID, &p.OwnerID, &p.Name, &p.Mode, &p.SiteURL, &enabled, &confirmed, &p.CreatedAt, &p.LastSuccessAt, &p.LastError)
	p.Enabled = enabled != 0
	p.Confirmed = confirmed != 0
	return p, e
}
func (s *Service) Peer(ctx context.Context, id string) (Peer, error) {
	return scanPeer(s.db.QueryRowContext(ctx, s.q(`SELECT `+peerColumns+` FROM chat_federation_peers WHERE id=?`), id))
}
func (s *Service) Peers(ctx context.Context, tenant, domain string) ([]Peer, error) {
	rows, e := s.db.QueryContext(ctx, s.q(`SELECT `+peerColumns+` FROM chat_federation_peers WHERE tenant_id=? AND domain_id=? ORDER BY name,id`), tenant, domain)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Peer{}
	for rows.Next() {
		p, e := scanPeer(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Service) CreatePeer(ctx context.Context, p Peer, c Credentials) (Peer, error) {
	if p.ID == "" {
		p.ID = model.NewId()
	}
	if !model.IsValidId(p.ID) || p.TenantID == "" || p.DomainID == "" || len(p.Name) < 1 || len(p.Name) > 64 || (p.Mode != "native" && p.Mode != "rest") {
		return Peer{}, ErrInvalid
	}
	for _, c := range p.Name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_') {
			return Peer{}, ErrInvalid
		}
	}
	if s.ScopeAllowed != nil && !s.ScopeAllowed(ctx, p.TenantID, p.DomainID) {
		return Peer{}, ErrForbidden
	}
	domain, e := s.Store.GetDomainByID(ctx, p.DomainID)
	if e != nil || domain.TenantID != p.TenantID {
		return Peer{}, ErrForbidden
	}
	owner, e := s.Store.GetUserByID(ctx, p.OwnerID)
	if e != nil || !owner.Enabled || owner.TenantID != p.TenantID || owner.DomainID != p.DomainID {
		return Peer{}, ErrForbidden
	}
	p.SiteURL, e = siteURL(p.SiteURL)
	if p.SiteURL == strings.TrimRight(s.PublicURL, "/") {
		return Peer{}, ErrInvalid
	}
	if e != nil {
		return Peer{}, e
	}
	if p.Mode == "rest" && (c.AccessToken == "" || len(c.AccessToken) > 4096 || strings.ContainsAny(c.AccessToken, "\r\n\x00")) {
		return Peer{}, ErrInvalid
	}
	if p.Mode == "native" && c.Token == "" {
		c.Token = model.NewId()
	}
	p.CreatedAt = time.Now().UnixMilli()
	p.Enabled = true
	p.Confirmed = p.Mode == "rest"
	secret, e := s.seal(p, c)
	if e != nil {
		return Peer{}, e
	}
	confirmed := 0
	if p.Confirmed {
		confirmed = 1
	}
	_, e = s.db.ExecContext(ctx, s.q(`INSERT INTO chat_federation_peers(id,tenant_id,domain_id,owner_id,name,mode,site_url,secret,enabled,confirmed,created_at) VALUES(?,?,?,?,?,?,?,?,1,?,?)`), p.ID, p.TenantID, p.DomainID, p.OwnerID, p.Name, p.Mode, p.SiteURL, secret, confirmed, p.CreatedAt)
	return p, e
}
func (s *Service) Disable(ctx context.Context, p Peer) error {
	_, e := s.db.ExecContext(ctx, s.q(`UPDATE chat_federation_peers SET enabled=0,secret='' WHERE id=? AND tenant_id=? AND domain_id=?`), p.ID, p.TenantID, p.DomainID)
	return e
}

const linkColumns = "id,peer_id,channel_id,remote_channel_id,enabled,readonly,created_at,local_cursor_at,local_cursor_id,remote_cursor_at,remote_cursor_id,local_readonly"

func scanLink(row interface{ Scan(...any) error }) (Link, error) {
	var l Link
	var en, ro, localRO int
	e := row.Scan(&l.ID, &l.PeerID, &l.ChannelID, &l.RemoteChannelID, &en, &ro, &l.CreatedAt, &l.LocalCursorAt, &l.LocalCursorID, &l.RemoteCursorAt, &l.RemoteCursorID, &localRO)
	l.Enabled = en != 0
	l.ReadOnly = ro != 0
	l.LocalReadOnly = localRO != 0
	return l, e
}
func (s *Service) Links(ctx context.Context, p Peer) ([]Link, error) {
	rows, e := s.db.QueryContext(ctx, s.q(`SELECT `+linkColumns+` FROM chat_federation_links WHERE peer_id=? ORDER BY id`), p.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Link{}
	for rows.Next() {
		l, e := scanLink(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
func (s *Service) Link(ctx context.Context, p Peer, id string) (Link, error) {
	return scanLink(s.db.QueryRowContext(ctx, s.q(`SELECT `+linkColumns+` FROM chat_federation_links WHERE peer_id=? AND id=?`), p.ID, id))
}
func (s *Service) CreateLink(ctx context.Context, p Peer, l Link, backfill bool) (Link, error) {
	if !p.Enabled || !p.Confirmed || !model.IsValidId(l.RemoteChannelID) {
		return Link{}, ErrInvalid
	}
	owner, e := s.Store.GetUserByID(ctx, p.OwnerID)
	if e != nil {
		return Link{}, e
	}
	c, e := s.Chat.Channel(ctx, owner, l.ChannelID)
	if e != nil || c.TenantID != p.TenantID || c.Kind == "D" {
		return Link{}, ErrForbidden
	}
	creator, e := s.Store.GetUserByID(ctx, c.CreatorID)
	if e != nil || creator.DomainID != p.DomainID {
		return Link{}, ErrForbidden
	}
	if e = s.channelScope(ctx, p, l.ChannelID); e != nil {
		return Link{}, e
	}
	l.ID = model.NewId()
	l.PeerID = p.ID
	l.CreatedAt = time.Now().UnixMilli()
	l.Enabled = true
	if !backfill {
		l.LocalCursorAt = l.CreatedAt
		l.RemoteCursorAt = l.CreatedAt
	}
	ro := 0
	if l.ReadOnly {
		ro = 1
	}
	existing, lookupErr := scanLink(s.db.QueryRowContext(ctx, s.q(`SELECT `+linkColumns+` FROM chat_federation_links WHERE peer_id=? AND channel_id=?`), p.ID, l.ChannelID))
	if lookupErr == nil {
		if existing.Enabled || existing.RemoteChannelID != l.RemoteChannelID {
			return Link{}, ErrInvalid
		}
		l.ID = existing.ID
		_, e = s.db.ExecContext(ctx, s.q(`UPDATE chat_federation_links SET enabled=1,readonly=?,local_readonly=0,local_cursor_at=?,local_cursor_id='',remote_cursor_at=?,remote_cursor_id='',lease_token='',lease_until=0 WHERE id=? AND peer_id=? AND enabled=0`), ro, l.LocalCursorAt, l.RemoteCursorAt, l.ID, p.ID)
		return l, e
	}
	if !errors.Is(lookupErr, sql.ErrNoRows) {
		return Link{}, lookupErr
	}
	_, e = s.db.ExecContext(ctx, s.q(`INSERT INTO chat_federation_links(id,peer_id,channel_id,remote_channel_id,enabled,readonly,created_at,local_cursor_at,remote_cursor_at) VALUES(?,?,?,?,1,?,?,?,?)`), l.ID, p.ID, l.ChannelID, l.RemoteChannelID, ro, l.CreatedAt, l.LocalCursorAt, l.RemoteCursorAt)
	return l, e
}
func (s *Service) DisableLink(ctx context.Context, p Peer, id string) error {
	_, e := s.db.ExecContext(ctx, s.q(`UPDATE chat_federation_links SET enabled=0 WHERE id=? AND peer_id=?`), id, p.ID)
	return e
}
func (s *Service) active(ctx context.Context, p Peer, l Link) error {
	if s.ScopeAllowed != nil && !s.ScopeAllowed(ctx, p.TenantID, p.DomainID) {
		return ErrForbidden
	}
	owner, e := s.Store.GetUserByID(ctx, p.OwnerID)
	if e != nil || !owner.Enabled || owner.TenantID != p.TenantID || owner.DomainID != p.DomainID {
		return ErrForbidden
	}
	if _, e = s.Chat.Channel(ctx, owner, l.ChannelID); e != nil {
		return ErrForbidden
	}

	if e = s.channelScope(ctx, p, l.ChannelID); e != nil {
		return e
	}
	var count int
	if l.LeaseToken != "" {
		var token string
		if err := s.db.QueryRowContext(ctx, s.q(`SELECT lease_token FROM chat_federation_links WHERE id=? AND peer_id=?`), l.ID, p.ID).Scan(&token); err != nil || token != l.LeaseToken {
			return ErrForbidden
		}
	}
	e = s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM chat_federation_links l JOIN chat_federation_peers p ON p.id=l.peer_id WHERE l.id=? AND p.id=? AND p.tenant_id=? AND p.domain_id=? AND p.enabled=1 AND p.confirmed=1 AND l.enabled=1`), l.ID, p.ID, p.TenantID, p.DomainID).Scan(&count)
	if e != nil {
		return e
	}
	if count != 1 {
		return ErrForbidden
	}
	return nil
}

type PendingInvite struct {
	ChannelID string `json:"remote_channel_id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	CreatedAt int64  `json:"create_at"`
}

func (s *Service) Pending(ctx context.Context, p Peer) ([]PendingInvite, error) {
	rows, e := s.db.QueryContext(ctx, s.q(`SELECT remote_channel_id,name,kind,created_at FROM chat_federation_invites WHERE peer_id=? ORDER BY created_at,remote_channel_id`), p.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []PendingInvite{}
	for rows.Next() {
		var v PendingInvite
		if e = rows.Scan(&v.ChannelID, &v.Name, &v.Kind, &v.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Service) Channels(ctx context.Context, p Peer, owner *storage.User) ([]map[string]string, error) {
	if owner.ID != p.OwnerID || owner.TenantID != p.TenantID || owner.DomainID != p.DomainID {
		return nil, ErrForbidden
	}
	channels, e := s.Chat.Channels(ctx, owner)
	if e != nil {
		return nil, e
	}
	out := []map[string]string{}
	for _, c := range channels {
		creator, e := s.Store.GetUserByID(ctx, c.CreatorID)
		if e == nil && creator.DomainID == p.DomainID && c.Kind != "D" {
			out = append(out, map[string]string{"id": c.ID, "name": c.Name, "display_name": c.DisplayName})
		}
	}
	return out, nil
}

// Existing tenant-wide channels may contain members from other domains. Such
// channels need an explicit domain-local membership before enabling federation.
func (s *Service) channelScope(ctx context.Context, p Peer, channelID string) error {
	var count int
	err := s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM chat_members m JOIN users u ON u.id=m.user_id WHERE m.channel_id=? AND (u.tenant_id<>? OR u.domain_id<>?)`), channelID, p.TenantID, p.DomainID).Scan(&count)
	if err != nil {
		return err
	}
	if count != 0 {
		return ErrForbidden
	}
	return nil
}
