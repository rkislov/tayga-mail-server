package webpush

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	push "github.com/SherClockHolmes/webpush-go"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/notify"
	"github.com/tayga/tms/internal/storage"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testService(t *testing.T) *Service {
	t.Helper()
	cfg := config.StorageConfig{Driver: "sqlite"}
	cfg.SQLite.Path = filepath.Join(t.TempDir(), "push.db")
	driver, err := storage.Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { driver.Close() })
	s, err := New(context.Background(), driver.(*storage.Store), "mail.example.com")
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func subscription(t *testing.T) push.Subscription {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return push.Subscription{Endpoint: "https://fcm.googleapis.com/fcm/send/test", Keys: push.Keys{Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16)), P256dh: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())}}
}
func TestOwnershipQueueAndPersistentKeys(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	sub := subscription(t)
	if err := s.Subscribe(ctx, "alice", sub); err != nil {
		t.Fatal(err)
	}
	if err := s.Subscribe(ctx, "bob", sub); err == nil {
		t.Fatal("endpoint reassigned")
	}
	event := notify.Event{ID: "event", Kind: notify.KindMail, Title: "private subject", Body: "private body", Href: "mail"}
	if err := s.Enqueue("alice", event); err != nil {
		t.Fatal(err)
	}
	if err := s.Enqueue("alice", event); err != nil {
		t.Fatal(err)
	}
	var count int
	var payload string
	s.db.QueryRow("SELECT COUNT(*),payload FROM webpush_jobs").Scan(&count, &payload)
	if count != 1 || strings.Contains(payload, "private") {
		t.Fatalf("count=%d payload=%s", count, payload)
	}
	if err := s.Unsubscribe(ctx, "bob", sub.Endpoint); err != nil {
		t.Fatal(err)
	}
	s.db.QueryRow("SELECT COUNT(*) FROM webpush_jobs").Scan(&count)
	if count != 1 {
		t.Fatal("foreign user deleted queue")
	}
	again, err := New(ctx, storage.NewSQLStore(s.db, storage.DialectSQLite), "mail.example.com")
	if err != nil || again.public != s.public || again.private != s.private {
		t.Fatal("keys changed on restart", err)
	}
	if err = s.Unsubscribe(ctx, "alice", sub.Endpoint); err != nil {
		t.Fatal(err)
	}
	s.db.QueryRow("SELECT COUNT(*) FROM webpush_jobs").Scan(&count)
	if count != 0 {
		t.Fatal("queued jobs survived unsubscribe")
	}
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestDeliveryRetriesAndExpiredSubscription(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	if err := s.Subscribe(ctx, "alice", subscription(t)); err != nil {
		t.Fatal(err)
	}
	if err := s.Enqueue("alice", notify.Event{ID: "a", Kind: "chat"}); err != nil {
		t.Fatal(err)
	}
	status := 503
	s.client = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "vapid ") {
			t.Error("missing VAPID auth")
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	s.deliver(ctx)
	var attempts int
	var next int64
	if err := s.db.QueryRow("SELECT attempts,next_attempt FROM webpush_jobs").Scan(&attempts, &next); err != nil || attempts != 1 || next <= time.Now().Unix() {
		t.Fatal("retry not scheduled", err, attempts, next)
	}
	status = 410
	s.db.Exec("UPDATE webpush_jobs SET next_attempt=0")
	s.deliver(ctx)
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM webpush_subscriptions").Scan(&count)
	if count != 0 {
		t.Fatal("expired subscription retained")
	}
	s.db.QueryRow("SELECT COUNT(*) FROM webpush_jobs").Scan(&count)
	if count != 0 {
		t.Fatal("expired job retained")
	}
}
func TestRejectUnsafeEndpointsAndKeys(t *testing.T) {
	sub := subscription(t)
	for _, endpoint := range []string{"https://127.0.0.1/push", "http://fcm.googleapis.com/push", "https://fcm.googleapis.com.evil.test/push", "https://fcm.googleapis.com:8443/push", "https://user@fcm.googleapis.com/push"} {
		sub.Endpoint = endpoint
		if Validate(sub) == nil {
			t.Fatal(endpoint)
		}
	}
	sub = subscription(t)
	sub.Keys.P256dh = "invalid"
	if Validate(sub) == nil {
		t.Fatal("invalid key accepted")
	}
}
