package xmpp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/tayga/tms/internal/storage"
)

// BotMessage is a deferred/inbound message for HTTP bots.
type BotMessage struct {
	ID      string    `json:"id"`
	From    string    `json:"from"`
	Body    string    `json:"body"`
	Stanza  string    `json:"stanza,omitempty"`
	Created time.Time `json:"created_at"`
}

// BotInfo is returned when creating/listing bots.
type BotInfo struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Email       string `json:"email,omitempty"`
	Name        string `json:"name"`
	TokenPrefix string `json:"token_prefix"`
	WebhookURL  string `json:"webhook_url,omitempty"`
	Enabled     bool   `json:"enabled"`
	Token       string `json:"token,omitempty"` // only on create
}

type botRow struct {
	ID          string
	UserID      string
	Name        string
	TokenHash   string
	TokenPrefix string
	WebhookURL  string
	Enabled     bool
}

// Gateway exposes bot + web messenger operations to the HTTP API.
type Gateway interface {
	CreateBot(ctx context.Context, userID, name, webhookURL string) (*BotInfo, error)
	ListBots(ctx context.Context) ([]BotInfo, error)
	DeleteBot(ctx context.Context, botID string) error
	LookupBotToken(ctx context.Context, token string) (*BotInfo, error)
	SendBotMessage(ctx context.Context, botUserEmail, to, body, msgType string) error
	PollBotInbox(ctx context.Context, botID string, limit int) ([]BotMessage, error)
	ComponentDomains() []string

	ListChatRoster(ctx context.Context, userID string) ([]RosterEntry, error)
	UpsertChatRoster(ctx context.Context, userID, jid, name string) error
	ChatHistory(ctx context.Context, ownerBare, withBare string, limit int) ([]HistoryMsg, error)
	SendChatMessage(ctx context.Context, fromEmail, to, body string) (ChatEvent, error)
	SubscribeChat(bare string) (<-chan ChatEvent, func())
}

func (s *Server) ComponentDomains() []string {
	if s.cfg == nil {
		return nil
	}
	var out []string
	for _, c := range s.cfg.XMPP.Components {
		if d := s.componentDomain(c); d != "" {
			out = append(out, d)
		}
	}
	return out
}

func (s *Server) CreateBot(ctx context.Context, userID, name, webhookURL string) (*BotInfo, error) {
	if s.db == nil {
		return nil, fmt.Errorf("xmpp storage unavailable")
	}
	u, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	token, prefix, hash, err := newBotToken()
	if err != nil {
		return nil, err
	}
	id := storage.NewID()
	_, err = s.db.ExecContext(ctx, s.rebind(`
		INSERT INTO xmpp_bots (id, user_id, name, token_hash, token_prefix, webhook_url, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?)`),
		id, userID, name, hash, prefix, webhookURL, time.Now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	return &BotInfo{
		ID: id, UserID: userID, Email: u.Email, Name: name,
		TokenPrefix: prefix, WebhookURL: webhookURL, Enabled: true, Token: token,
	}, nil
}

func (s *Server) ListBots(ctx context.Context) ([]BotInfo, error) {
	if s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT b.id, b.user_id, b.name, b.token_prefix, b.webhook_url, b.enabled, u.email
		FROM xmpp_bots b JOIN users u ON u.id = b.user_id
		ORDER BY b.created_at`) //nolint:gosec // static query
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BotInfo
	for rows.Next() {
		var b BotInfo
		var en int
		if err := rows.Scan(&b.ID, &b.UserID, &b.Name, &b.TokenPrefix, &b.WebhookURL, &en, &b.Email); err != nil {
			return nil, err
		}
		b.Enabled = en != 0
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Server) DeleteBot(ctx context.Context, botID string) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_bots WHERE id = ?`), botID)
	return err
}

func (s *Server) LookupBotToken(ctx context.Context, token string) (*BotInfo, error) {
	if s.db == nil || token == "" {
		return nil, sql.ErrNoRows
	}
	prefix := token
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(`
		SELECT b.id, b.user_id, b.name, b.token_hash, b.token_prefix, b.webhook_url, b.enabled, u.email
		FROM xmpp_bots b JOIN users u ON u.id = b.user_id
		WHERE b.token_prefix = ? AND b.enabled = 1`), prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	want := hashToken(token)
	for rows.Next() {
		var br botRow
		var email string
		var en int
		if err := rows.Scan(&br.ID, &br.UserID, &br.Name, &br.TokenHash, &br.TokenPrefix, &br.WebhookURL, &en, &email); err != nil {
			return nil, err
		}
		if br.TokenHash == want {
			return &BotInfo{
				ID: br.ID, UserID: br.UserID, Email: email, Name: br.Name,
				TokenPrefix: br.TokenPrefix, WebhookURL: br.WebhookURL, Enabled: en != 0,
			}, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *Server) SendBotMessage(ctx context.Context, botUserEmail, to, body, msgType string) error {
	if msgType == "" {
		msgType = "chat"
	}
	from := ParseJID(botUserEmail).Bare()
	toJ := ParseJID(to)
	id := randomID()[:12]
	bodyXML := ""
	if body != "" {
		bodyXML = `<body>` + xmlEscape(body) + `</body>`
	}
	stanza := fmt.Sprintf(
		`<message from='%s' to='%s' id='%s' type='%s'>%s</message>`,
		xmlEscape(from), xmlEscape(toJ.Full()), xmlEscape(id), xmlEscape(msgType), bodyXML,
	)
	b := []byte(stanza)
	if !s.hub.Route(toJ.Bare(), b) && !s.hub.Route(to, b) {
		_ = s.storeOffline(ctx, toJ.Bare(), string(b))
	}
	_ = s.archiveMAM(ctx, from, toJ.Bare(), id, string(b))
	_ = s.archiveMAM(ctx, toJ.Bare(), from, id, string(b))
	s.notifyWebFromStanza(stanza, toJ.Bare())
	return nil
}

func (s *Server) PollBotInbox(ctx context.Context, botID string, limit int) ([]BotMessage, error) {
	if s.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(`
		SELECT id, from_jid, stanza, body, created_at FROM xmpp_bot_inbox
		WHERE bot_id = ? ORDER BY created_at LIMIT ?`), botID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BotMessage
	var ids []string
	for rows.Next() {
		var m BotMessage
		if err := rows.Scan(&m.ID, &m.From, &m.Stanza, &m.Body, &m.Created); err != nil {
			return nil, err
		}
		out = append(out, m)
		ids = append(ids, m.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		_, _ = s.db.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_bot_inbox WHERE id = ?`), id)
	}
	return out, nil
}

func (s *Server) enqueueBotInbox(ctx context.Context, toBare, from, stanza, body string) error {
	if s.db == nil {
		return nil
	}
	var botID string
	err := s.db.QueryRowContext(ctx, s.rebind(`
		SELECT b.id FROM xmpp_bots b
		JOIN users u ON u.id = b.user_id
		WHERE lower(u.email) = lower(?) AND b.enabled = 1`), toBare,
	).Scan(&botID)
	if err != nil {
		return nil // not a bot mailbox
	}
	id := storage.NewID()
	_, err = s.db.ExecContext(ctx, s.rebind(`
		INSERT INTO xmpp_bot_inbox (id, bot_id, from_jid, stanza, body, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`),
		id, botID, from, stanza, body, time.Now().UTC(),
	)
	return err
}

func newBotToken() (token, prefix, hash string, err error) {
	var buf [24]byte
	if _, err = rand.Read(buf[:]); err != nil {
		return
	}
	token = "tbot_" + hex.EncodeToString(buf[:])
	prefix = token[:8]
	hash = hashToken(token)
	return
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
