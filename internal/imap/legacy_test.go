package imapserver

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-imap/commands"
	"github.com/emersion/go-imap/responses"
	imapserv "github.com/emersion/go-imap/server"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/testutil/mailfixture"
)

func TestLegacyAPPENDRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	storageCfg := config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}}
	if dsn := os.Getenv("TAYGA_TEST_IMAP_POSTGRES_DSN"); dsn != "" {
		// Opt-in PostgreSQL run uses a fresh schema; existing tables are untouched.
		u, err := url.Parse(dsn)
		if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
			t.Fatal("test DSN must be a PostgreSQL URL")
		}
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			t.Fatal(err)
		}
		schema := "imap_legacy_" + strings.ReplaceAll(storage.NewID(), "-", "")
		if _, err = db.Exec("CREATE SCHEMA " + schema); err != nil {
			db.Close()
			t.Fatal(err)
		}
		t.Cleanup(func() {
			defer db.Close()
			if _, err := db.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
				t.Error(err)
			}
		})
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		storageCfg = config.StorageConfig{Driver: "postgres", Postgres: config.PostgresConfig{DSN: u.String()}}
	}
	st, err := storage.Open(ctx, storageCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tenant, err := st.CreateTenant(ctx, "Legacy mail")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := st.CreateDomain(ctx, tenant.ID, "example.test")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := (auth.Argon2id{}).Hash("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	user, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: "recipient", Email: "recipient@example.test", PasswordHash: hash, Enabled: true, AuthSource: "local"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	ms := mailstore.New(filepath.Join(dir, "mail"))
	be := &Backend{store: st, ms: ms, hub: NewHub(st), log: slog.New(slog.NewTextHandler(io.Discard, nil)), authn: auth.NewLayer(st, cfg.LDAP, cfg.MFA, cfg.OIDC)}
	// This round-trip does not test IDLE. go-imap v1's update dispatcher reads
	// the selected mailbox concurrently with SELECT; keep that unrelated
	// upstream race out of the charset fixture.
	srv := imapserv.New(struct{ backend.Backend }{be})
	srv.Enable(charsetSearchExtension{})
	srv.AllowInsecureAuth = true
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go srv.Serve(listener)
	c, err := client.Dial(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c.Timeout = 10 * time.Second
	defer c.Logout()
	if err = c.Login(user.Email, "fixture-password"); err != nil {
		t.Fatal(err)
	}
	// CREATE/LIST/SELECT also exercise IMAP modified UTF-7 folder names.
	const folder = "Перенесённая почта"
	if err = c.Create(folder); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2023, 12, 19, 8, 15, 0, 0, time.UTC)
	var originals [][]byte
	for _, cs := range mailfixture.Charsets {
		raw := mailfixture.Message(t, cs.Label, "base64", true)
		if err = c.Append(folder, []string{imap.SeenFlag, imap.FlaggedFlag}, when, bytes.NewReader(raw)); err != nil {
			t.Fatalf("APPEND %s: %v", cs.Label, err)
		}
		originals = append(originals, raw)
	}
	if _, err = c.Select(folder, true); err != nil {
		t.Fatal(err)
	}
	seq := new(imap.SeqSet)
	seq.AddRange(1, uint32(len(originals)))
	section := &imap.BodySectionName{Peek: true}
	ch := make(chan *imap.Message, len(originals))
	err = c.Fetch(seq, []imap.FetchItem{section.FetchItem(), imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, imap.FetchRFC822Size}, ch)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for msg := range ch {
		count++
		want := originals[msg.SeqNum-1]
		got, err := io.ReadAll(msg.GetBody(section))
		if err != nil || !bytes.Equal(got, want) || msg.Size != uint32(len(want)) {
			t.Fatalf("FETCH changed RFC822 message %d: %v", msg.SeqNum, err)
		}
		if msg.Envelope.Subject != mailfixture.Subject || len(msg.Envelope.From) != 1 || msg.Envelope.From[0].PersonalName != mailfixture.Sender || !msg.InternalDate.Equal(when) || !slices.Contains(msg.Flags, imap.SeenFlag) || !slices.Contains(msg.Flags, imap.FlaggedFlag) {
			t.Fatalf("FETCH metadata: %+v, envelope %+v", msg, msg.Envelope)
		}
	}
	if count != len(originals) {
		t.Fatalf("FETCH returned %d messages", count)
	}
	for _, cs := range mailfixture.Charsets {
		criteria := imap.NewSearchCriteria()
		criteria.Header.Set("Subject", string(mailfixture.Encode(t, cs.Label, mailfixture.Subject)))
		for _, uid := range []bool{false, true} {
			result := new(responses.Search)
			var cmd imap.Commander = &commands.Search{Charset: cs.Label, Criteria: criteria}
			if uid {
				cmd = &commands.Uid{Cmd: cmd}
			}
			status, err := c.Execute(cmd, result)
			if err != nil || status.Err() != nil || len(result.Ids) != len(originals) {
				t.Fatalf("SEARCH CHARSET %s uid=%v: result=%v status=%v error=%v", cs.Label, uid, result.Ids, status, err)
			}
		}
	}
	criteria := imap.NewSearchCriteria()
	criteria.Header.Set("Subject", "missing")
	status, err := c.Execute(&commands.Search{Charset: "x-unknown", Criteria: criteria}, new(responses.Search))
	if err != nil || status.Type != imap.StatusRespNo || status.Code != imap.CodeBadCharset {
		t.Fatalf("unknown SEARCH CHARSET response: %v, %v", status, err)
	}
	mb, err := st.GetMailbox(ctx, user.ID, folder)
	if err != nil {
		t.Fatal(err)
	}
	hits, err := st.SearchMessages(ctx, user.ID, mb.ID, "", "", "", "октябрь", 50)
	if err != nil || len(hits) != len(originals) {
		t.Fatalf("Cyrillic full-text search returned %d: %v", len(hits), err)
	}
	msgs, err := st.ListMessages(ctx, mb.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range msgs {
		data, err := ms.Read(msg.FilePath)
		if err != nil || !slices.ContainsFunc(originals, func(raw []byte) bool { return bytes.Equal(raw, data) }) {
			t.Fatalf("stored bytes changed: %v", err)
		}
	}
}

func TestIMAPSearchUnknownCharsetKeepsHeaders(t *testing.T) {
	ms := mailstore.New(t.TempDir())
	raw := []byte("Subject: Imported message\r\nContent-Type: text/plain; charset=x-unknown\r\n\r\nASCII body")
	path, _, err := ms.Deliver("u@example.test", "INBOX", raw)
	if err != nil {
		t.Fatal(err)
	}
	criteria := imap.NewSearchCriteria()
	criteria.Header.Set("Subject", "Imported")
	ok, err := matchMessage(ms, &storage.Message{FilePath: path, UID: 1}, 1, criteria)
	if err != nil || !ok {
		t.Fatalf("unknown body charset lost searchable headers: %v", err)
	}
}

func TestLegacyEnvelopeNamesAreNotAddressSyntax(t *testing.T) {
	name := "Петров, Иван <отдел>"
	word := mime.BEncoding.Encode("win-1251", string(mailfixture.Encode(t, "win-1251", name)))
	var raw strings.Builder
	for _, field := range []string{"From", "To", "Sender", "Reply-To", "Cc", "Bcc"} {
		raw.WriteString(field + ": " + word + " <person@example.test>\r\n")
	}
	raw.WriteString("\r\n")
	h, _, err := headerAndBody([]byte(raw.String()))
	if err != nil {
		t.Fatal(err)
	}
	env := fetchEnvelope(h)
	for _, list := range [][]*imap.Address{env.From, env.To, env.Sender, env.ReplyTo, env.Cc, env.Bcc} {
		if len(list) != 1 || list[0].PersonalName != name || list[0].MailboxName != "person" || list[0].HostName != "example.test" {
			t.Fatalf("encoded display name changed address list: %+v", list)
		}
	}
}
