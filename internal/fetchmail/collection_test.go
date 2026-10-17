package fetchmail

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/migrate"
	"github.com/tayga/tms/internal/storage"
)

func fixture(t *testing.T) (*Service, *storage.User, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	cfg := config.Default()
	cfg.Storage.Driver = "sqlite"
	cfg.Storage.SQLite.Path = filepath.Join(dir, "db")
	cfg.Mailstore.Root = filepath.Join(dir, "maildir")
	st, e := storage.Open(ctx, cfg.Storage)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	tenant, e := st.CreateTenant(ctx, "collector")
	if e != nil {
		t.Fatal(e)
	}
	dom, e := st.CreateDomain(ctx, tenant.ID, "example.com")
	if e != nil {
		t.Fatal(e)
	}
	u, e := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: dom.ID, Email: "owner@example.com", LocalPart: "owner", Enabled: true, AuthSource: "local"})
	if e != nil {
		t.Fatal(e)
	}
	svc, e := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), st, mailstore.New(cfg.Mailstore.Root))
	if e != nil {
		t.Fatal(e)
	}
	return svc, u, cfg
}
func settings(protocol string) Settings {
	return Settings{Name: "External", Protocol: protocol, Host: "external.example", Port: 143, Security: "plain", Username: "external-user", Password: "external-secret", SourceFolder: "INBOX", TargetFolder: "Imported", Interval: 15, Enabled: true}
}
func save(t *testing.T, s *Service, u *storage.User, protocol string) *Source {
	t.Helper()
	a, e := s.Save(context.Background(), u.ID, "", settings(protocol))
	if e != nil {
		t.Fatal(e)
	}
	return a
}

const rawMessage = "From: Sender <sender@example.org>\r\nTo: owner@example.com\r\nSubject: External mail\r\nMessage-ID: <external@example.org>\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nHello\r\n.dot line\r\n"

func mockDial(s *Service, serve func(net.Conn)) {
	s.dial = func(ctx context.Context, _ string, _ int) (net.Conn, error) {
		a, b := net.Pipe()
		b.SetDeadline(time.Now().Add(5 * time.Second))
		go func() { defer b.Close(); serve(b) }()
		return a, nil
	}
}
func popServer(retrieved *atomic.Int32, authOK bool) func(net.Conn) {
	return func(c net.Conn) {
		r := bufio.NewReader(c)
		io.WriteString(c, "+OK ready\r\n")
		for {
			line, e := r.ReadString('\n')
			if e != nil {
				return
			}
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "USER "):
				io.WriteString(c, "+OK\r\n")
			case strings.HasPrefix(line, "PASS "):
				if authOK {
					io.WriteString(c, "+OK\r\n")
				} else {
					io.WriteString(c, "-ERR rejected external-secret\r\n")
				}
			case line == "UIDL":
				io.WriteString(c, "+OK\r\n1 stable-uid\r\n.\r\n")
			case line == "LIST 1":
				fmt.Fprintf(c, "+OK 1 %d\r\n", len(rawMessage))
			case line == "RETR 1":
				retrieved.Add(1)
				io.WriteString(c, "+OK\r\n")
				w := textproto.NewWriter(bufio.NewWriter(c)).DotWriter()
				w.Write([]byte(rawMessage))
				w.Close()
			case line == "QUIT":
				io.WriteString(c, "+OK bye\r\n")
				return
			default:
				io.WriteString(c, "-ERR unexpected command\r\n")
			}
		}
	}
}
func imapServer(retrieved *atomic.Int32, validity uint32) func(net.Conn) {
	return func(c net.Conn) {
		r := bufio.NewReader(c)
		io.WriteString(c, "* OK [CAPABILITY IMAP4rev1] ready\r\n")
		for {
			line, e := r.ReadString('\n')
			if e != nil {
				return
			}
			parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
			if len(parts) != 2 {
				return
			}
			tag, cmd := parts[0], parts[1]
			switch {
			case cmd == "CAPABILITY":
				io.WriteString(c, "* CAPABILITY IMAP4rev1\r\n")
			case strings.HasPrefix(cmd, "LOGIN "):
			case strings.HasPrefix(cmd, "EXAMINE "):
				fmt.Fprintf(c, "* FLAGS (\\Seen \\Flagged)\r\n* 1 EXISTS\r\n* OK [UIDVALIDITY %d] stable\r\n", validity)
			case strings.HasPrefix(cmd, "UID SEARCH"):
				io.WriteString(c, "* SEARCH 42\r\n")
			case strings.HasPrefix(cmd, "UID FETCH 42 "):
				if strings.Contains(cmd, "BODY.PEEK[]") {
					retrieved.Add(1)
					fmt.Fprintf(c, "* 1 FETCH (UID 42 BODY[] {%d}\r\n%s)\r\n", len(rawMessage), rawMessage)
				} else {
					fmt.Fprintf(c, "* 1 FETCH (UID 42 RFC822.SIZE %d FLAGS (\\Seen \\Flagged) INTERNALDATE \"17-Oct-2026 10:30:00 +0300\")\r\n", len(rawMessage))
				}
			default:
				fmt.Fprintf(c, "%s BAD unsupported\r\n", tag)
				continue
			}
			fmt.Fprintf(c, "%s OK completed\r\n", tag)
		}
	}
}
func TestCollectionDeduplicatesAfterRestartAndLocalDeletion(t *testing.T) {
	for _, protocol := range []string{"imap", "pop3"} {
		t.Run(protocol, func(t *testing.T) {
			s, u, cfg := fixture(t)
			a := save(t, s, u, protocol)
			var retrieved atomic.Int32
			serve := popServer(&retrieved, true)
			if protocol == "imap" {
				serve = imapServer(&retrieved, 77)
			}
			mockDial(s, serve)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			n, more, e := s.collect(ctx, a, false)
			if e != nil || n != 1 || more {
				t.Fatalf("first: %d %t %v", n, more, e)
			}
			boxes, e := s.store.ListMailboxes(ctx, u.ID)
			if e != nil || len(boxes) != 1 || boxes[0].Name != "Imported" {
				t.Fatalf("boxes: %v %v", boxes, e)
			}
			msgs, e := s.store.ListMessages(ctx, boxes[0].ID)
			if e != nil || len(msgs) != 1 {
				t.Fatalf("messages: %v %v", msgs, e)
			}
			data, e := s.ms.Read(msgs[0].FilePath)
			if e != nil || string(data) != rawMessage {
				t.Fatalf("MIME did not survive: %v", e)
			}
			if protocol == "imap" && (!strings.Contains(msgs[0].Flags, `\Seen`) || msgs[0].InternalDate.UTC().Hour() != 7) {
				t.Fatalf("metadata: %+v", msgs[0])
			}
			if e = s.store.DeleteMessage(ctx, msgs[0].ID); e != nil {
				t.Fatal(e)
			}
			restarted, e := New(cfg, s.log, s.store, s.ms)
			if e != nil {
				t.Fatal(e)
			}
			mockDial(restarted, serve)
			saved, e := restarted.Get(ctx, u.ID, a.ID)
			if e != nil {
				t.Fatal(e)
			}
			n, _, e = restarted.collect(ctx, saved, false)
			if e != nil || n != 0 || retrieved.Load() != 1 {
				t.Fatalf("duplicate: %d fetched=%d %v", n, retrieved.Load(), e)
			}
			if protocol == "imap" {
				mockDial(restarted, imapServer(&retrieved, 78))
				n, _, e = restarted.collect(ctx, saved, false)
				if e != nil || n != 1 {
					t.Fatalf("changed UIDVALIDITY: %d %v", n, e)
				}
			}
		})
	}
}
func TestCollectionSettingsCredentialsAndLeases(t *testing.T) {
	s, u, _ := fixture(t)
	ctx := context.Background()
	a := save(t, s, u, "pop3")
	data, _ := json.Marshal(a)
	if bytes.Contains(data, []byte("secret")) || bytes.Contains(a.PasswordCiphertext, []byte("external-secret")) {
		t.Fatal("credential leaked")
	}
	if _, e := s.Get(ctx, "other", a.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	swapped := *a
	swapped.UserID = "other"
	if _, e := s.password(&swapped); e == nil {
		t.Fatal("ciphertext not owner-bound")
	}
	in := settings("pop3")
	in.Password = ""
	updated, e := s.Save(ctx, u.ID, a.ID, in)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.password(updated)
	if e != nil || p != "external-secret" {
		t.Fatal("password not preserved", e)
	}
	claimed, e := s.claim(ctx)
	if e != nil || claimed == nil {
		t.Fatalf("claim: %v", e)
	}
	if next, e := s.claim(ctx); e != nil || next != nil {
		t.Fatalf("duplicate lease: %v %v", next, e)
	}
	if _, e = s.Save(ctx, u.ID, a.ID, in); !errors.Is(e, ErrBusy) {
		t.Fatal("edited busy source", e)
	}
	if e = s.Delete(ctx, u.ID, a.ID); !errors.Is(e, ErrBusy) {
		t.Fatal("deleted busy source", e)
	}
	var retrieved atomic.Int32
	mockDial(s, popServer(&retrieved, true))
	s.run(ctx, claimed)
	saved, e := s.Get(ctx, u.ID, a.ID)
	if e != nil || saved.Status != "ok" || saved.Copied != 1 || saved.LeaseUntil != 0 || saved.NextRun < time.Now().Add(14*time.Minute).Unix() {
		t.Fatalf("schedule: %+v %v", saved, e)
	}
	if e = s.RunNow(ctx, u.ID, a.ID); e != nil {
		t.Fatal(e)
	}
	if next, e := s.claim(ctx); e != nil || next == nil {
		t.Fatalf("run now: %v", e)
	}
}
func TestConnectionTestDoesNotImportAndRedactsFailures(t *testing.T) {
	s, u, _ := fixture(t)
	var retrieved atomic.Int32
	mockDial(s, popServer(&retrieved, true))
	in := settings("pop3")
	if e := s.Test(context.Background(), u.ID, "", in); e != nil {
		t.Fatal(e)
	}
	boxes, _ := s.store.ListMailboxes(context.Background(), u.ID)
	if len(boxes) != 0 || retrieved.Load() != 0 {
		t.Fatal("connection test imported mail")
	}
	mockDial(s, popServer(&retrieved, false))
	if e := s.Test(context.Background(), u.ID, "", in); e == nil || strings.Contains(e.Error(), in.Password) {
		t.Fatal("authentication failure leaked credential", e)
	}
	mockDial(s, func(c net.Conn) { io.Copy(io.Discard, c) })
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	if e := s.Test(ctx, u.ID, "", in); e == nil || time.Since(start) > time.Second {
		t.Fatal("cancellation not honored", e)
	}
}
func TestPublicAddressValidation(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "169.254.169.254", "100.64.0.1", "::1", "::ffff:127.0.0.1", "fc00::1", "64:ff9b::a00:1"} {
		if publicIP(net.ParseIP(ip)) {
			t.Fatal("accepted private", ip)
		}
	}
	for _, ip := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		if !publicIP(net.ParseIP(ip)) {
			t.Fatal("rejected public", ip)
		}
	}
}
func TestIMAPFramingLimits(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		fail        bool
	}{
		{"literal", "* 1 FETCH (BODY[] {11}\r\na\r\n{9000}\nx)\r\n", false},
		{"oversize", "* 1 FETCH (BODY[] {1000000}\r\n", true},
		{"line", strings.Repeat("x", imapLineLimit+1) + "\r\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			go func() { defer b.Close(); io.WriteString(b, tc.input) }()
			out, e := io.ReadAll(limitIMAP(a, 32))
			if tc.fail && e == nil {
				t.Fatal("accepted excessive response")
			}
			if !tc.fail && (e != nil || string(out) != tc.input) {
				t.Fatalf("wire changed: %q %v", out, e)
			}
		})
	}
}

func TestTLSAndSTARTTLSCollection(t *testing.T) {
	certificateServer := httptest.NewTLSServer(nil)
	cert := certificateServer.TLS.Certificates[0]
	roots := x509.NewCertPool()
	roots.AddCert(certificateServer.Certificate())
	certificateServer.Close()
	for _, protocol := range []string{"imap", "pop3"} {
		for _, security := range []string{"tls", "starttls"} {
			t.Run(protocol+"/"+security, func(t *testing.T) {
				s, u, _ := fixture(t)
				in := settings(protocol)
				in.Security = security
				in.Host = "example.com"
				a, e := s.Save(context.Background(), u.ID, "", in)
				if e != nil {
					t.Fatal(e)
				}
				s.tlsConfig = func(a *Source) *tls.Config { c := tlsConfig(a); c.RootCAs = roots; return c }
				var retrieved atomic.Int32
				serve := popServer(&retrieved, true)
				if protocol == "imap" {
					serve = imapServer(&retrieved, 77)
				}
				mockDial(s, func(conn net.Conn) {
					if security == "starttls" {
						r := bufio.NewReader(conn)
						if protocol == "imap" {
							io.WriteString(conn, "* OK [CAPABILITY IMAP4rev1 STARTTLS] ready\r\n")
							line, e := r.ReadString('\n')
							if e != nil || line != "collector_tls STARTTLS\r\n" {
								return
							}
							io.WriteString(conn, "collector_tls OK begin TLS\r\n")
						} else {
							io.WriteString(conn, "+OK ready\r\n")
							line, e := r.ReadString('\n')
							if e != nil || line != "STLS\r\n" {
								return
							}
							io.WriteString(conn, "+OK begin TLS\r\n")
						}
					}
					tc := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
					if tc.Handshake() != nil {
						return
					}
					if security == "starttls" {
						serve(&discardGreetingConn{Conn: tc})
					} else {
						serve(tc)
					}
				})
				n, _, e := s.collect(context.Background(), a, false)
				if e != nil || n != 1 {
					t.Fatalf("secure collection: %d %v", n, e)
				}
			})
		}
	}
	t.Run("reject untrusted certificate", func(t *testing.T) {
		s, u, _ := fixture(t)
		in := settings("pop3")
		in.Security = "tls"
		a, e := s.Save(context.Background(), u.ID, "", in)
		if e != nil {
			t.Fatal(e)
		}
		mockDial(s, func(c net.Conn) {
			tc := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{cert}})
			tc.Handshake()
		})
		_, _, e = s.collect(context.Background(), a, true)
		if e == nil || !strings.Contains(e.Error(), "TLS verification") {
			t.Fatal("accepted untrusted certificate", e)
		}
	})
}

// A protocol greeting was already consumed before STARTTLS; the regular fixture
// emits one on start, so omit only that fixture write after the TLS handshake.
type discardGreetingConn struct {
	net.Conn
	discarded bool
}

func (c *discardGreetingConn) Write(p []byte) (int, error) {
	if !c.discarded {
		c.discarded = true
		return len(p), nil
	}
	return c.Conn.Write(p)
}

func TestCollectionSizeAndQuotaFailureDoNotRecordReceipt(t *testing.T) {
	s, u, _ := fixture(t)
	a := save(t, s, u, "pop3")
	var fetched atomic.Int32
	mockDial(s, popServer(&fetched, true))
	s.maxSize = 10
	if n, _, e := s.collect(context.Background(), a, false); e == nil || n != 0 || fetched.Load() != 0 {
		t.Fatal("size limit failed", n, e)
	}
	s.maxSize = 1 << 20
	if _, e := s.db.Exec(`UPDATE users SET quota_bytes=10 WHERE id=?`, u.ID); e != nil {
		t.Fatal(e)
	}
	if n, _, e := s.collect(context.Background(), a, false); !errors.Is(e, storage.ErrQuotaExceeded) || n != 0 {
		t.Fatal("quota limit failed", n, e)
	}
	var receipts int
	if e := s.db.QueryRow(`SELECT COUNT(*) FROM mail_source_receipts`).Scan(&receipts); e != nil || receipts != 0 {
		t.Fatal("failed import marked complete", receipts, e)
	}
}

func TestRelativeMaildirUsesExistingSecretDirectory(t *testing.T) {
	s, _, cfg := fixture(t)
	t.Chdir(t.TempDir())
	cfg.Mailstore.Root = "maildir"
	existing, e := migrate.ResolveKey("", "./data")
	if e != nil {
		t.Fatal(e)
	}
	collector, e := New(cfg, s.log, s.store, s.ms)
	if e != nil {
		t.Fatal(e)
	}
	expected := sha256.Sum256(append(existing, []byte("tayga-mail-collection-v1")...))
	if !bytes.Equal(collector.key, expected[:]) {
		t.Fatal("key directory differs from migration service")
	}
	if _, e = os.Stat("secrets.key"); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("created key in working directory", e)
	}
}
