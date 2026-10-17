package spam

import (
	"context"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLearningMovesCorrectionsRetriesAndIsolation(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	driver, e := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(root, "db")}})
	if e != nil {
		t.Fatal(e)
	}
	defer driver.Close()
	st := driver.(*storage.Store)
	tenant, _ := st.CreateTenant(ctx, "test")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "example.com")
	user, e := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "owner@example.com", LocalPart: "owner", Enabled: true})
	if e != nil {
		t.Fatal(e)
	}
	inbox, _ := st.EnsureMailbox(ctx, user.ID, "INBOX", "")
	junk, _ := st.EnsureMailbox(ctx, user.ID, "Junk", "")
	trash, _ := st.EnsureMailbox(ctx, user.ID, "Trash", "")
	ms := mailstore.New(filepath.Join(root, "mail"))
	raw := []byte("From: sender@example.net\r\nSubject: Training fixture\r\n\r\ncontents\r\n")
	path, size, e := ms.Deliver(user.Email, "INBOX", raw)
	if e != nil {
		t.Fatal(e)
	}
	msg, e := st.InsertMessage(ctx, &storage.Message{MailboxID: inbox.ID, FilePath: path, Size: size})
	if e != nil {
		t.Fatal(e)
	}
	var received []string
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != string(raw) || r.Header.Get("User") != user.Email || r.Header.Get("Password") != "test-controller-password" {
			t.Error("invalid learning payload")
		}
		received = append(received, r.URL.Path)
		if fail {
			w.WriteHeader(503)
			io.WriteString(w, `{"error":"offline"}`)
			return
		}
		io.WriteString(w, `{"success":true}`)
	}))
	defer server.Close()
	cfg := config.SpamConfig{Enabled: true, Backend: "rspamd"}
	cfg.Learning.URL = server.URL
	cfg.Learning.Password = "test-controller-password"
	st.SetSpamLearningPolicy(func() (bool, string) { return LearningEnabled(cfg), "Junk" })
	worker := &LearningWorker{Store: st, Mailstore: ms, Config: func() config.SpamConfig { return cfg }, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	worker.Process(ctx)
	if len(received) != 0 {
		t.Fatal("initial delivery trained")
	}
	move := func(id string) {
		t.Helper()
		if _, e = st.MoveMessage(ctx, msg.ID, id); e != nil {
			t.Fatal(e)
		}
	}
	move(junk.ID)
	worker.Process(ctx)
	move(inbox.ID)
	worker.Process(ctx)
	if strings.Join(received, ",") != "/learnspam,/learnham" {
		t.Fatal(received)
	}
	move(junk.ID)
	jobs, _ := st.DueSpamLearning(ctx)
	if len(jobs) != 1 {
		t.Fatal(jobs)
	}
	claimed, e := st.ClaimSpamLearning(ctx, jobs[0])
	if e != nil || !claimed {
		t.Fatal(claimed, e)
	}
	move(inbox.ID)
	if due, _ := st.DueSpamLearning(ctx); len(due) != 0 {
		t.Fatal("correction overtook in-flight request")
	}
	if e = st.FinishSpamLearning(ctx, jobs[0], "spam", ""); e != nil {
		t.Fatal(e)
	}
	worker.Process(ctx)
	move(junk.ID)
	fail = true
	worker.Process(ctx)
	if due, _ := st.DueSpamLearning(ctx); len(due) != 0 {
		t.Fatal("retry not delayed")
	}
	st.DB().Exec(`UPDATE spam_learning_jobs SET next_attempt=0`)
	fail = false
	worker.Process(ctx)
	before := len(received)
	move(trash.ID)
	worker.Process(ctx)
	if len(received) != before {
		t.Fatal("deletion trained ham")
	}
	cfg.Learning.Disabled = true
	move(junk.ID)
	worker.Process(ctx)
	if len(received) != before {
		t.Fatal("disabled learning sent request")
	}
	stats, e := st.ProtectionStatistics(ctx, []string{domain.ID}, time.Now().Add(-time.Hour).Unix()/3600*3600)
	if e != nil {
		t.Fatal(e)
	}
	counts := stats["counts"].(map[string]map[string]int64)["learning"]
	if counts["spam"] != 3 || counts["ham"] != 2 || counts["error"] != 1 {
		t.Fatal(counts)
	}
	foreign, e := st.ProtectionStatistics(ctx, []string{"foreign"}, 0)
	if e != nil || len(foreign["counts"].(map[string]map[string]int64)["learning"]) != 0 {
		t.Fatal(foreign, e)
	}
	cfg.Learning.Disabled = false
	copy := &storage.Message{MailboxID: inbox.ID, FilePath: path, Size: size}
	copy, e = st.InsertCopiedMessage(ctx, copy, junk.ID)
	if e != nil {
		t.Fatal(e)
	}
	worker.Process(ctx)
	if received[len(received)-1] != "/learnham" {
		t.Fatal("IMAP copy feedback missing")
	}
}
func TestLearningControllerDefaultsAndFailures(t *testing.T) {
	cfg := config.SpamConfig{URL: "http://127.0.0.1:11333"}
	c, e := Controller(cfg)
	if e != nil || c.base != "http://127.0.0.1:11334" {
		t.Fatal(c, e)
	}
	for _, address := range []string{"file:///tmp/secret", "https://u:p@host", "http://host/?password=secret"} {
		cfg.Learning.URL = address
		if _, e = Controller(cfg); e == nil {
			t.Fatal(address)
		}
	}
	for _, tc := range []struct {
		body, want string
		code       int
	}{{`{"success":true}`, "spam", 200}, {`{"error":"already learned"}`, "duplicate", 200}, {`{"error":"denied"}`, "", 403}, {`<html>login</html>`, "", 200}} {
		t.Run(tc.body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.code); io.WriteString(w, tc.body) }))
			defer srv.Close()
			r := NewRspamd(srv.URL, "", time.Second)
			out, e := r.Learn(context.Background(), "owner@example.com", "spam", []byte("fixture"))
			if out != tc.want || (e == nil) != (tc.want != "") {
				t.Fatal(out, e)
			}
		})
	}
}
