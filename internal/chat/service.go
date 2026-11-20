// Package chat implements the protocol-independent, persistent TMS messenger.
package chat

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tayga/tms/internal/storage"
)

var (
	ErrForbidden = errors.New("chat access denied")
	ErrInvalid   = errors.New("invalid chat request")
	channelName  = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)
)

// ID maps internal identifiers to stable 26-character client identifiers.
func ID(value string) string {
	h := sha256.Sum256([]byte(value))
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(h[:16]))
}
func newID() string { return ID(uuid.NewString()) }

type Channel struct {
	LastPostAt, TotalMsgCount, RootMsgCount          int64
	ID, TenantID, Name, DisplayName, Kind, CreatorID string
	CreatedAt                                        int64
}
type Post struct {
	ReplyCount                                        int64
	ID, ChannelID, UserID, RootID, Message, PendingID string
	CreatedAt                                         int64
}
type Member struct {
	UserID                  string
	LastViewedAt, ReadCount int64
}
type Event struct{ Post Post }

type Service struct {
	store       storage.Driver
	db          *sql.DB
	postgres    bool
	mu          sync.Mutex
	subscribers map[chan Event]struct{}
}

func New(store storage.Driver) *Service {
	s := &Service{store: store, db: store.DB(), subscribers: make(map[chan Event]struct{})}
	if d, ok := store.(interface{ DialectName() string }); ok {
		s.postgres = d.DialectName() == "postgres"
	}
	return s
}
func (s *Service) q(query string) string {
	if !s.postgres {
		return query
	}
	n := 0
	return rebind(query, &n)
}
func rebind(q string, n *int) string {
	var b strings.Builder
	for _, r := range q {
		if r == '?' {
			(*n)++
			fmt.Fprintf(&b, "$%d", *n)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (s *Service) User(ctx context.Context, tenantID, id string) (*storage.User, error) {
	users, err := s.store.ListUsersByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.Enabled && ID(u.ID) == id {
			return u, nil
		}
	}
	return nil, storage.ErrNotFound
}
func scanChannel(row interface{ Scan(...any) error }) (Channel, error) {
	var c Channel
	err := row.Scan(&c.ID, &c.TenantID, &c.Name, &c.DisplayName, &c.Kind, &c.CreatorID, &c.CreatedAt, &c.LastPostAt, &c.TotalMsgCount, &c.RootMsgCount)
	if errors.Is(err, sql.ErrNoRows) {
		err = storage.ErrNotFound
	}
	return c, err
}

const channelColumns = "c.id,c.tenant_id,c.name,c.display_name,c.kind,c.creator_id,c.created_at,(SELECT COALESCE(MAX(p.created_at),0) FROM chat_posts p WHERE p.channel_id=c.id),(SELECT COUNT(*) FROM chat_posts p WHERE p.channel_id=c.id),(SELECT COUNT(*) FROM chat_posts p WHERE p.channel_id=c.id AND p.root_id='')"

func (s *Service) Channel(ctx context.Context, u *storage.User, id string) (Channel, error) {
	return scanChannel(s.db.QueryRowContext(ctx, s.q(`SELECT `+channelColumns+` FROM chat_channels c JOIN chat_members m ON m.channel_id=c.id WHERE c.id=? AND c.tenant_id=? AND m.user_id=?`), id, u.TenantID, u.ID))
}
func (s *Service) Channels(ctx context.Context, u *storage.User) ([]Channel, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT `+channelColumns+` FROM chat_channels c JOIN chat_members m ON m.channel_id=c.id WHERE c.tenant_id=? AND m.user_id=? ORDER BY c.created_at,c.id`), u.TenantID, u.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Channel{}
	for rows.Next() {
		c, e := scanChannel(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// EnsureDefault joins the tenant's common channel, atomically and idempotently.
func (s *Service) EnsureDefault(ctx context.Context, u *storage.User) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	id := ID("town-square:" + u.TenantID)
	_, err = tx.ExecContext(ctx, s.q(`INSERT INTO chat_channels(id,tenant_id,name,display_name,kind,creator_id,created_at) VALUES(?,?,?,'Town Square','O',?,?) ON CONFLICT DO NOTHING`), id, u.TenantID, "town-square", u.ID, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, s.q(`INSERT INTO chat_members(channel_id,user_id) VALUES(?,?) ON CONFLICT DO NOTHING`), id, u.ID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) CreateChannel(ctx context.Context, u *storage.User, name, display, kind string) (Channel, error) {
	if !channelName.MatchString(name) || len(display) == 0 || len(display) > 64 || (kind != "O" && kind != "P") {
		return Channel{}, ErrInvalid
	}
	// Only EnsureDefault may create this reserved channel.
	if name == "town-square" {
		return Channel{}, ErrInvalid
	}
	return s.create(ctx, u, newID(), name, display, kind, []string{u.ID})
}
func (s *Service) create(ctx context.Context, u *storage.User, id, name, display, kind string, members []string) (Channel, error) {
	c := Channel{ID: id, TenantID: u.TenantID, Name: name, DisplayName: display, Kind: kind, CreatorID: u.ID, CreatedAt: time.Now().UnixMilli()}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return c, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, s.q(`INSERT INTO chat_channels(id,tenant_id,name,display_name,kind,creator_id,created_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`), c.ID, c.TenantID, c.Name, c.DisplayName, c.Kind, c.CreatorID, c.CreatedAt)
	if err != nil {
		return c, err
	}
	// Resolve a concurrent creation, and reject name collisions without joining it.
	existing, err := scanChannel(tx.QueryRowContext(ctx, s.q(`SELECT `+channelColumns+` FROM chat_channels c WHERE c.tenant_id=? AND c.name=?`), u.TenantID, name))
	if err != nil {
		return c, err
	}
	if existing.ID != id {
		return c, ErrInvalid
	}
	c = existing
	for _, member := range members {
		_, err = tx.ExecContext(ctx, s.q(`INSERT INTO chat_members(channel_id,user_id) VALUES(?,?) ON CONFLICT DO NOTHING`), c.ID, member)
		if err != nil {
			return c, err
		}
	}
	return c, tx.Commit()
}
func (s *Service) Direct(ctx context.Context, u *storage.User, otherID string) (Channel, error) {
	other, err := s.User(ctx, u.TenantID, otherID)
	if err != nil {
		return Channel{}, err
	}
	ids := []string{ID(u.ID), ID(other.ID)}
	sort.Strings(ids)
	name := strings.Join(ids, "__")
	return s.create(ctx, u, ID("direct:"+u.TenantID+":"+name), name, "", "D", []string{u.ID, other.ID})
}
func (s *Service) AddMember(ctx context.Context, u *storage.User, channelID, userID string) error {
	c, err := s.Channel(ctx, u, channelID)
	if err != nil {
		return err
	}
	target, err := s.User(ctx, u.TenantID, userID)
	if err != nil {
		return err
	}
	if c.Kind == "D" || (c.Kind == "P" && c.CreatorID != u.ID) {
		return ErrForbidden
	}
	_, err = s.db.ExecContext(ctx, s.q(`INSERT INTO chat_members(channel_id,user_id) VALUES(?,?) ON CONFLICT DO NOTHING`), channelID, target.ID)
	return err
}
func (s *Service) Members(ctx context.Context, u *storage.User, channelID string) ([]Member, error) {
	if _, err := s.Channel(ctx, u, channelID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT m.user_id,m.last_viewed_at,(SELECT COUNT(*) FROM chat_posts p WHERE p.channel_id=m.channel_id AND p.created_at<=m.last_viewed_at) FROM chat_members m WHERE m.channel_id=? ORDER BY m.user_id`), channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.LastViewedAt, &m.ReadCount); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Service) View(ctx context.Context, u *storage.User, channelID string) error {
	if _, err := s.Channel(ctx, u, channelID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, s.q(`UPDATE chat_members SET last_viewed_at=? WHERE channel_id=? AND user_id=?`), time.Now().UnixMilli(), channelID, u.ID)
	return err
}
func scanPost(row interface{ Scan(...any) error }) (Post, error) {
	var p Post
	err := row.Scan(&p.ID, &p.ChannelID, &p.UserID, &p.RootID, &p.Message, &p.CreatedAt, &p.PendingID, &p.ReplyCount)
	if errors.Is(err, sql.ErrNoRows) {
		err = storage.ErrNotFound
	}
	return p, err
}

const postColumns = "id,channel_id,user_id,root_id,message,created_at,pending_id,(SELECT COUNT(*) FROM chat_posts replies WHERE replies.root_id=chat_posts.id)"

func (s *Service) Post(ctx context.Context, u *storage.User, id string) (Post, error) {
	p, err := scanPost(s.db.QueryRowContext(ctx, s.q(`SELECT `+postColumns+` FROM chat_posts WHERE id=?`), id))
	if err != nil {
		return p, err
	}
	_, err = s.Channel(ctx, u, p.ChannelID)
	return p, err
}
func (s *Service) Posts(ctx context.Context, u *storage.User, channelID string, limit, offset int, since int64) ([]Post, error) {
	if _, err := s.Channel(ctx, u, channelID); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 200 || offset < 0 {
		return nil, ErrInvalid
	}
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT `+postColumns+` FROM chat_posts WHERE channel_id=? AND created_at>? ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`), channelID, since, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Post{}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Service) Send(ctx context.Context, u *storage.User, channelID, rootID, message, pendingID string) (Post, error) {
	if _, err := s.Channel(ctx, u, channelID); err != nil {
		return Post{}, err
	}
	if strings.TrimSpace(message) == "" || len(message) > 65535 || len(pendingID) > 128 {
		return Post{}, ErrInvalid
	}
	if rootID != "" {
		root, err := s.Post(ctx, u, rootID)
		if err != nil {
			return Post{}, err
		}
		if root.ChannelID != channelID || root.RootID != "" {
			return Post{}, ErrInvalid
		}
	}
	p := Post{ID: newID(), ChannelID: channelID, UserID: u.ID, RootID: rootID, Message: message, PendingID: pendingID, CreatedAt: time.Now().UnixMilli()}
	result, err := s.db.ExecContext(ctx, s.q(`INSERT INTO chat_posts(id,channel_id,user_id,root_id,message,created_at,pending_id) VALUES(?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`), p.ID, p.ChannelID, p.UserID, p.RootID, p.Message, p.CreatedAt, p.PendingID)
	if err != nil {
		return Post{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return Post{}, err
	}
	if n == 0 {
		old, err := scanPost(s.db.QueryRowContext(ctx, s.q(`SELECT `+postColumns+` FROM chat_posts WHERE user_id=? AND pending_id=?`), u.ID, pendingID))
		if err != nil {
			return Post{}, err
		}
		if old.ChannelID != channelID || old.RootID != rootID || old.Message != message {
			return Post{}, ErrInvalid
		}
		return old, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subscribers {
		select {
		case ch <- Event{p}:
		default:
			close(ch)
			delete(s.subscribers, ch)
		}
	}
	return p, nil
}

// Subscribe exposes committed events. The protocol adapter must check channel
// membership before delivery. Slow consumers are closed and must resync history.
func (s *Service) Subscribe() (<-chan Event, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan Event, 64)
	s.subscribers[ch] = struct{}{}
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
	}
}

func (s *Service) Thread(ctx context.Context, u *storage.User, postID string) ([]Post, error) {
	p, err := s.Post(ctx, u, postID)
	if err != nil {
		return nil, err
	}
	root := p.ID
	if p.RootID != "" {
		root = p.RootID
	}
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT `+postColumns+` FROM chat_posts WHERE id=? OR root_id=? ORDER BY created_at,id LIMIT 200`), root, root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Post{}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) Unread(ctx context.Context, u *storage.User) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM chat_posts p JOIN chat_channels c ON c.id=p.channel_id JOIN chat_members m ON m.channel_id=p.channel_id WHERE c.tenant_id=? AND m.user_id=? AND p.created_at>m.last_viewed_at AND p.user_id<>?`), u.TenantID, u.ID, u.ID).Scan(&count)
	return count, err
}

// ServerID is the persistent installation identity used by native clients.
// INSERT ... DO NOTHING keeps concurrent adapters from replacing one another's ID.
func (s *Service) ServerID(ctx context.Context) (string, error) {
	const key = "mattermost.diagnostic_id"
	value, ok, err := s.store.GetSetting(ctx, key)
	if err != nil {
		return "", err
	}
	if ok && value != "" {
		return value, nil
	}
	_, err = s.db.ExecContext(ctx, s.q(`INSERT INTO settings(key,value,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO NOTHING`), key, newID(), time.Now().UTC())
	if err != nil {
		return "", err
	}
	value, ok, err = s.store.GetSetting(ctx, key)
	if err != nil {
		return "", err
	}
	if !ok || value == "" {
		return "", errors.New("chat installation identity is empty")
	}
	return value, nil
}
