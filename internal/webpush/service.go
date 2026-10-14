// Package webpush delivers private, generic notifications to browser subscriptions.
package webpush

import (
	"context"
	"crypto/ecdh"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	push "github.com/SherClockHolmes/webpush-go"
	"github.com/tayga/tms/internal/notify"
	"github.com/tayga/tms/internal/storage"
)

type Service struct {
	subscriptionsMu          sync.Mutex
	db                       *sql.DB
	postgres                 bool
	public, private, subject string
	client                   *http.Client
}

func (s *Service) query(q string) string {
	if !s.postgres {
		return q
	}
	n := 0
	for strings.Contains(q, "?") {
		n++
		q = strings.Replace(q, "?", fmt.Sprintf("$%d", n), 1)
	}
	return q
}

func New(ctx context.Context, store *storage.Store, hostname string) (*Service, error) {
	s := &Service{db: store.DB(), postgres: store.DialectName() == "postgres", subject: "https://" + hostname, client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("push redirects forbidden") }}}
	private, public, err := push.GenerateVAPIDKeys()
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx, s.query("INSERT INTO webpush_keys(id,public_key,private_key) VALUES(1,?,?) ON CONFLICT(id) DO NOTHING"), public, private)
	if err != nil {
		return nil, err
	}
	err = s.db.QueryRowContext(ctx, "SELECT public_key,private_key FROM webpush_keys WHERE id=1").Scan(&s.public, &s.private)
	return s, err
}
func (s *Service) PublicKey() string { return s.public }

// Restrict delivery to browser vendors; never send authenticated requests to arbitrary URLs.
func Validate(sub push.Subscription) error {
	u, err := url.Parse(sub.Endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Port() != "" || len(sub.Endpoint) > 4096 {
		return errors.New("invalid push endpoint")
	}
	h := strings.ToLower(u.Hostname())
	allowed := h == "fcm.googleapis.com" || h == "updates.push.services.mozilla.com" || h == "push.services.mozilla.com" || strings.HasSuffix(h, ".push.services.mozilla.com") || h == "web.push.apple.com" || strings.HasSuffix(h, ".push.apple.com")
	if !allowed {
		return errors.New("unsupported browser push service")
	}
	auth, err := base64.RawURLEncoding.DecodeString(sub.Keys.Auth)
	if err != nil || len(auth) != 16 {
		return errors.New("invalid auth key")
	}
	key, err := base64.RawURLEncoding.DecodeString(sub.Keys.P256dh)
	if err != nil {
		return errors.New("invalid encryption key")
	}
	if _, err = ecdh.P256().NewPublicKey(key); err != nil {
		return errors.New("invalid encryption key")
	}
	return nil
}
func (s *Service) Subscribe(ctx context.Context, user string, sub push.Subscription) error {
	s.subscriptionsMu.Lock()
	defer s.subscriptionsMu.Unlock()
	if err := Validate(sub); err != nil {
		return err
	}
	b, _ := json.Marshal(sub)
	var count int
	if err := s.db.QueryRowContext(ctx, s.query("SELECT COUNT(*) FROM webpush_subscriptions WHERE user_id=?"), user).Scan(&count); err != nil {
		return err
	}
	var owner string
	_ = s.db.QueryRowContext(ctx, s.query("SELECT user_id FROM webpush_subscriptions WHERE endpoint=?"), sub.Endpoint).Scan(&owner)
	if count >= 20 && owner != user {
		return errors.New("subscription limit reached")
	}
	result, err := s.db.ExecContext(ctx, s.query("INSERT INTO webpush_subscriptions(endpoint,user_id,subscription) VALUES(?,?,?) ON CONFLICT(endpoint) DO UPDATE SET subscription=excluded.subscription WHERE webpush_subscriptions.user_id=excluded.user_id"), sub.Endpoint, user, string(b))
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("subscription belongs to another account")
	}
	return nil
}
func (s *Service) Unsubscribe(ctx context.Context, user, endpoint string) error {
	s.subscriptionsMu.Lock()
	defer s.subscriptionsMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, s.query("DELETE FROM webpush_jobs WHERE endpoint=? AND EXISTS (SELECT 1 FROM webpush_subscriptions WHERE endpoint=? AND user_id=?)"), endpoint, endpoint, user)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, s.query("DELETE FROM webpush_subscriptions WHERE endpoint=? AND user_id=?"), endpoint, user)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) Enqueue(user string, ev notify.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	title := "Новое уведомление"
	switch ev.Kind {
	case notify.KindMail:
		title = "Новое письмо"
	case notify.KindCalendar:
		title = "Напоминание о событии"
	case "chat":
		title = "Новое сообщение"
	}
	if ev.ID == "" {
		ev.ID = storage.NewID()
	}
	payload, _ := json.Marshal(map[string]string{"title": title, "tag": ev.ID, "href": ev.Href})
	_, err := s.db.ExecContext(ctx, s.query("INSERT INTO webpush_jobs(id,endpoint,payload,next_attempt) SELECT ? || endpoint,endpoint,?,? FROM webpush_subscriptions WHERE user_id=? AND (SELECT COUNT(*) FROM webpush_jobs)<10000 ON CONFLICT(id) DO NOTHING"), ev.ID, string(payload), time.Now().Unix(), user)
	return err
}
func (s *Service) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.deliver(ctx)
			}
		}
	}()
}
func (s *Service) deliver(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, s.query("SELECT j.id,j.endpoint,j.payload,j.attempts,s.subscription FROM webpush_jobs j LEFT JOIN webpush_subscriptions s ON j.endpoint=s.endpoint WHERE j.next_attempt<=? ORDER BY j.next_attempt LIMIT 20"), time.Now().Unix())
	if err != nil {
		return
	}
	type job struct {
		id, endpoint, payload string
		attempts              int
		sub                   sql.NullString
	}
	var jobs []job
	for rows.Next() {
		var j job
		if rows.Scan(&j.id, &j.endpoint, &j.payload, &j.attempts, &j.sub) == nil {
			jobs = append(jobs, j)
		}
	}
	rows.Close()
	for _, j := range jobs {
		if ctx.Err() != nil {
			return
		}
		var sub push.Subscription
		remove := !j.sub.Valid || json.Unmarshal([]byte(j.sub.String), &sub) != nil || Validate(sub) != nil
		if !remove {
			resp, err := push.SendNotificationWithContext(ctx, []byte(j.payload), &sub, &push.Options{HTTPClient: s.client, Subscriber: s.subject, VAPIDPublicKey: s.public, VAPIDPrivateKey: s.private, TTL: 3600})
			if err == nil {
				resp.Body.Close()
				remove = resp.StatusCode >= 200 && resp.StatusCode < 300
				if resp.StatusCode == 404 || resp.StatusCode == 410 {
					_, _ = s.db.ExecContext(ctx, s.query("DELETE FROM webpush_subscriptions WHERE endpoint=?"), j.endpoint)
					remove = true
				}
			}
		}
		if remove || j.attempts >= 4 {
			_, _ = s.db.ExecContext(ctx, s.query("DELETE FROM webpush_jobs WHERE id=?"), j.id)
		} else {
			_, _ = s.db.ExecContext(ctx, s.query("UPDATE webpush_jobs SET attempts=attempts+1,next_attempt=? WHERE id=?"), time.Now().Add(time.Duration(1<<j.attempts)*time.Minute).Unix(), j.id)
		}
	}
}

// UserIDs includes subscribed users even when no browser tab is open.
func (s *Service) UserIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT user_id FROM webpush_subscriptions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []string
	for rows.Next() {
		var user string
		if err := rows.Scan(&user); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}
