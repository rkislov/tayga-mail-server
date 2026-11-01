package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

type failingMailMove struct {
	storage.Driver
	fail bool
}

func (s *failingMailMove) MoveMessageContent(ctx context.Context, before *storage.Message, dst, path string, size int64, archived bool) (*storage.Message, error) {
	if s.fail {
		return nil, errors.New("injected database commit failure")
	}
	return s.Driver.MoveMessageContent(ctx, before, dst, path, size, archived)
}
func TestMailBatchArchiveFailureAndRetry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := config.Default()
	base, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	st := &failingMailMove{Driver: base}
	layer := auth.NewLayer(st, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("fixture-password")
	tenant, _ := st.CreateTenant(ctx, "fixture")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "example.test")
	user, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "user@example.test", LocalPart: "user", Enabled: true, AuthSource: "local", PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	ms := mailstore.New(filepath.Join(dir, "mail"))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httpapi.New(cfg, logger, st, layer, ms, nil, nil).Handler()
	login, err := layer.LoginWithPassword(ctx, user.Email, "fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Authorization", "Bearer "+login.Tokens.AccessToken)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/api/v1/mail/mailboxes", nil); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	inbox, _ := st.GetMailbox(ctx, user.ID, "INBOX")
	archive, _ := st.GetMailbox(ctx, user.ID, "Archive")
	trash, _ := st.GetMailbox(ctx, user.ID, "Trash")
	junk, _ := st.GetMailbox(ctx, user.ID, "Junk")
	raw := []byte("Message-ID: <fixture@example.test>\r\nSubject: Batch fixture\r\nFrom: fixture@example.test\r\n\r\noriginal body\r\n")
	create := func() *storage.Message {
		t.Helper()
		path, size, err := ms.Deliver(user.Email, "INBOX", raw)
		if err != nil {
			t.Fatal(err)
		}
		m, err := st.InsertMessage(ctx, &storage.Message{MailboxID: inbox.ID, FilePath: path, Size: size, Flags: `\Seen`, Subject: "Batch fixture"})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	m := create()
	st.fail = true
	w := call("POST", "/api/v1/mail/messages/"+m.ID+"/archive", nil)
	if w.Code != 503 || bytes.Contains(w.Body.Bytes(), []byte(dir)) {
		t.Fatalf("unsafe failure: %d %s", w.Code, w.Body.String())
	}
	after, _ := st.GetMessageByID(ctx, m.ID)
	if after.FilePath != m.FilePath || after.MailboxID != inbox.ID {
		t.Fatal("failed archive mutated original")
	}
	if data, e := ms.Read(m.FilePath); e != nil || !bytes.Equal(data, raw) {
		t.Fatal("original lost", e)
	}
	entries, _ := os.ReadDir(filepath.Join(ms.UserRoot(user.Email), ".Archive", "cur"))
	if len(entries) != 0 {
		t.Fatal("failed candidate not cleaned")
	}
	st.fail = false
	// Concurrent duplicate actions must compress once and keep one stable UID/path.
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- call("POST", "/api/v1/mail/messages/"+m.ID+"/archive", nil).Code }()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 200 {
			t.Fatalf("archive code %d", code)
		}
	}
	after, _ = st.GetMessageByID(ctx, m.ID)
	savedPath, savedUID := after.FilePath, after.UID
	if after.MailboxID != archive.ID || !after.Archived {
		t.Fatal("archive not committed")
	}
	if data, e := ms.Read(savedPath); e != nil || !bytes.Equal(data, raw) {
		t.Fatal("archive content", e)
	}
	call("POST", "/api/v1/mail/messages/"+m.ID+"/archive", nil)
	after, _ = st.GetMessageByID(ctx, m.ID)
	if after.FilePath != savedPath || after.UID != savedUID {
		t.Fatal("archive retry changed content identity")
	}
	if _, e := os.Stat(ms.Abs(m.FilePath)); !os.IsNotExist(e) {
		t.Fatal("original not cleaned after successful commit")
	}
	// Real transaction failure after the mailbox update must roll back folder+UID.
	store := base.(*storage.Store)
	beforeUID := junk.UIDNext
	_, err = store.DB().Exec(`CREATE TRIGGER fail_content BEFORE UPDATE OF file_path ON messages BEGIN SELECT RAISE(ABORT,'injected path failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	w = call("POST", "/api/v1/mail/messages/"+m.ID+"/move", map[string]string{"mailbox_id": junk.ID})
	if w.Code != 503 {
		t.Fatalf("expected failed content update: %s", w.Body.String())
	}
	_, _ = store.DB().Exec(`DROP TRIGGER fail_content`)
	after, _ = st.GetMessageByID(ctx, m.ID)
	junk, _ = st.GetMailbox(ctx, user.ID, "Junk")
	if after.MailboxID != archive.ID || after.FilePath != savedPath || junk.UIDNext != beforeUID {
		t.Fatal("partial folder/path/UID commit")
	}
	if _, err = st.MoveMessageContent(ctx, m, inbox.ID, "unused", 0, false); err != storage.ErrMessageChanged {
		t.Fatal("stale snapshot not rejected", err)
	}
	// A missing file and an unauthorized mailbox do not prevent the other 48 items.
	missing := create()
	_ = ms.Delete(missing.FilePath)
	items := []map[string]string{{"id": missing.ID, "mailbox_id": inbox.ID}}
	for range 48 {
		x := create()
		items = append(items, map[string]string{"id": x.ID, "mailbox_id": inbox.ID})
	}
	started := time.Now()
	w = call("POST", "/api/v1/mail/messages/batch", map[string]any{"action": "archive", "items": items})
	elapsed := time.Since(started)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var response struct {
		Results []struct {
			ID    string `json:"id"`
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		} `json:"results"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	success := 0
	for _, r := range response.Results {
		if r.OK {
			success++
		}
	}
	if len(response.Results) != 49 || success != 48 || response.Results[0].Error != "message_content_unavailable" {
		t.Fatalf("partial batch %s", w.Body.String())
	}
	t.Logf("49 items, 48 archived, one missing: %s", elapsed)
	// A failed trash move cannot hard-delete a message.
	x := create()
	_, err = store.DB().Exec(fmt.Sprintf(`CREATE TRIGGER fail_trash BEFORE UPDATE OF mailbox_id ON messages WHEN NEW.mailbox_id='%s' BEGIN SELECT RAISE(ABORT,'injected trash failure'); END`, trash.ID))
	if err != nil {
		t.Fatal(err)
	}
	w = call("DELETE", "/api/v1/mail/messages/"+x.ID, nil)
	if w.Code != 503 {
		t.Fatal(w.Body.String())
	}
	if _, err = st.GetMessageByID(ctx, x.ID); err != nil {
		t.Fatal("message deleted after failed trash move")
	}
	_, _ = store.DB().Exec(`DROP TRIGGER fail_trash`)
	// Retry of the same selected inbox item is a conflict, never a permanent delete.
	body := map[string]any{"action": "delete", "items": []map[string]string{{"id": x.ID, "mailbox_id": inbox.ID}}}
	for range 2 {
		w = call("POST", "/api/v1/mail/messages/batch", body)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	after, err = st.GetMessageByID(ctx, x.ID)
	if err != nil || after.MailboxID != trash.ID {
		t.Fatal("retry deleted trashed item", err)
	}
	other, _ := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "other@example.test", LocalPart: "other", Enabled: true, AuthSource: "local", PasswordHash: hash})
	otherbox, _ := st.EnsureMailbox(ctx, other.ID, "INBOX", ms.UserRoot(other.Email))
	foreign, _ := st.InsertMessage(ctx, &storage.Message{MailboxID: otherbox.ID, FilePath: "private", Flags: ""})
	w = call("POST", "/api/v1/mail/messages/batch", map[string]any{"action": "read", "items": []map[string]string{{"id": foreign.ID, "mailbox_id": otherbox.ID}, {"id": m.ID, "mailbox_id": archive.ID}}})
	after, _ = st.GetMessageByID(ctx, foreign.ID)
	if after.Flags != "" || !bytes.Contains(w.Body.Bytes(), []byte("message_not_found")) {
		t.Fatal("foreign message modified", w.Body.String())
	}
	for _, bad := range []any{map[string]any{"action": "archive", "items": []any{}}, map[string]any{"action": "invalid", "items": items}, map[string]any{"action": "archive", "items": []any{items[0], items[0]}}, map[string]any{"action": "archive", "items": append(items, items[0], items[0])}} {
		if w = call(http.MethodPost, "/api/v1/mail/messages/batch", bad); w.Code != 400 {
			t.Fatal("invalid request accepted")
		}
	}
}
