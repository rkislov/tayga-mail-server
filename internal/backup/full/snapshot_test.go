package full

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/maintenance"
	"github.com/tayga/tms/internal/migrate"
	"github.com/tayga/tms/internal/settings"
	"github.com/tayga/tms/internal/storage"
	"gopkg.in/yaml.v3"
)

type fullFixture struct {
	root, parent, cfgFile, output, keyFile, user, reader, event, message string
	cfg                                                                  *config.Config
	key                                                                  *age.X25519Identity
	files                                                                map[string]string
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newFullFixture(t *testing.T) *fullFixture {
	t.Helper()
	f := &fullFixture{parent: t.TempDir(), files: map[string]string{}}
	var err error
	f.parent, err = filepath.EvalSymlinks(f.parent)
	must(t, err)
	f.root = filepath.Join(f.parent, "installation")
	must(t, os.Mkdir(f.root, 0700))
	f.cfgFile = filepath.Join(f.root, "config.yaml")
	f.output = filepath.Join(f.parent, "snapshot.tar.gz.age")
	f.keyFile = filepath.Join(f.parent, "identity.txt")
	f.key, err = age.GenerateX25519Identity()
	must(t, err)
	must(t, os.WriteFile(f.keyFile, []byte(f.key.String()+"\n"), 0600))
	f.cfg = config.Default()
	f.cfg.Storage.SQLite.Path = filepath.Join(f.root, "tayga.db")
	f.cfg.Mailstore.Root = filepath.Join(f.root, "maildir")
	f.cfg.TLS.CertFile = filepath.Join(f.root, "certs", "server.crt")
	f.cfg.TLS.KeyFile = filepath.Join(f.root, "certs", "server.key")
	f.cfg.TLS.CertsDir = filepath.Join(f.root, "certs", "store")
	f.cfg.Log.File = filepath.Join(f.root, "tayga.log")
	must(t, os.MkdirAll(f.cfg.TLS.CertsDir, 0700))
	must(t, os.MkdirAll(f.cfg.Mailstore.Root, 0700))
	f.put(t, "certs/server.crt", "fixture certificate")
	f.put(t, "certs/server.key", "fixture private TLS key")
	f.put(t, "secrets.key", "fixture-persistent-server-key")
	f.put(t, "dkim/domain.pem", "fixture private DKIM key")
	cfgData, err := yaml.Marshal(f.cfg)
	must(t, err)
	must(t, os.WriteFile(f.cfgFile, cfgData, 0600))
	ctx := context.Background()
	driver, err := storage.Open(ctx, f.cfg.Storage)
	must(t, err)
	defer driver.Close()
	st := driver.(*storage.Store)
	ten, err := st.CreateTenant(ctx, "Isolated restore")
	must(t, err)
	domain, err := st.CreateDomain(ctx, ten.ID, "example.test")
	must(t, err)
	hash, err := (auth.Argon2id{}).Hash("fixture-password")
	must(t, err)
	owner, err := st.CreateUser(ctx, &storage.User{TenantID: ten.ID, DomainID: domain.ID, Email: "owner@example.test", LocalPart: "owner", DisplayName: "Restore Owner", Enabled: true, AuthSource: "local", PasswordHash: hash})
	must(t, err)
	reader, err := st.CreateUser(ctx, &storage.User{TenantID: ten.ID, DomainID: domain.ID, Email: "reader@example.test", LocalPart: "reader", Enabled: true, AuthSource: "local", PasswordHash: hash})
	must(t, err)
	f.user, f.reader = owner.ID, reader.ID
	mb, err := st.EnsureMailbox(ctx, owner.ID, "INBOX", filepath.Join(f.cfg.Mailstore.Root, owner.Email))
	must(t, err)
	f.put(t, "maildir/owner@example.test/cur/mail.eml", "From: sender@example.test\r\nSubject: backup fixture\r\n\r\nMail survives recovery.\r\n")
	msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: "owner@example.test/cur/mail.eml", MessageID: "fixture@example.test", Subject: "backup fixture", Size: 80, InternalDate: time.Now().UTC()})
	must(t, err)
	f.message = msg.ID
	must(t, st.SetMailboxACL(ctx, mb.ID, reader.ID, "lr"))
	cal, err := st.EnsureCalendar(ctx, owner.ID, "default", "Calendar")
	must(t, err)
	start, end := time.Now().UTC().Truncate(time.Second), time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	event, err := st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, HrefName: "fixture.ics", UID: "fixture-event", Component: "VEVENT", Data: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:fixture-event\r\nSUMMARY:Recovered meeting\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", DTStart: &start, DTEnd: &end})
	must(t, err)
	f.event = event.ID
	must(t, st.SetCalendarACL(ctx, cal.ID, reader.ID, "r"))
	f.put(t, "maildir/calendar-attachments/ticket.pdf", "%PDF-fixture-ticket")
	_, err = st.CreateCalendarAttachment(ctx, &storage.CalendarAttachment{CalendarObjectID: event.ID, UserID: owner.ID, Filename: "ticket.pdf", ContentType: "application/pdf", Size: 19, StoragePath: "ticket.pdf"})
	must(t, err)
	must(t, st.SaveTask(ctx, owner.ID, &storage.Task{Title: "Recovered task"}))
	f.put(t, "maildir/files/"+owner.ID+"/report.txt", "current file")
	f.put(t, "maildir/file-state/"+owner.ID+"/versions/rev/content", "previous file")
	f.put(t, "maildir/file-state/"+owner.ID+"/trash/deleted/content", "deleted file")
	must(t, st.SetFileAccess(ctx, owner.ID, "report.txt", reader.ID, "read"))
	_, err = st.DB().ExecContext(ctx, `INSERT INTO xmpp_mam(id,owner_bare_jid,with_bare_jid,stanza_id,stanza) VALUES ('message','owner@example.test','reader@example.test','stanza','<encrypted>fixture-ciphertext</encrypted>')`)
	must(t, err)
	_, err = st.DB().ExecContext(ctx, `INSERT INTO xmpp_pep_nodes(id,owner_bare_jid,node) VALUES ('node','owner@example.test','eu.siacs.conversations.axolotl.bundles:1')`)
	must(t, err)
	_, err = st.DB().ExecContext(ctx, `INSERT INTO xmpp_pep_items(id,node_id,item_id,payload) VALUES ('bundle','node','current','fixture public bundle')`)
	must(t, err)
	_, err = st.DB().ExecContext(ctx, `INSERT INTO webpush_keys(id,public_key,private_key) VALUES (1,'fixture-public','fixture-private')`)
	must(t, err)
	_, err = st.DB().ExecContext(ctx, `INSERT INTO web_chat_rooms(jid,name,owner_email) VALUES ('room@conference.example.test','Recovered room','owner@example.test')`)
	must(t, err)
	_, err = st.DB().ExecContext(ctx, `INSERT INTO web_chat_members(room_jid,email) VALUES ('room@conference.example.test','reader@example.test')`)
	must(t, err)
	binding, err := json.Marshal(storage.DKIMBinding{Domain: domain.Name, Selector: "test", KeyFile: filepath.Join(f.root, "dkim/domain.pem"), Enabled: true})
	must(t, err)
	must(t, st.PutSetting(ctx, "dkim.domain.example.test", string(binding)))
	return f
}

func (f *fullFixture) put(t *testing.T, rel, body string) {
	t.Helper()
	name := filepath.Join(f.root, filepath.FromSlash(rel))
	must(t, os.MkdirAll(filepath.Dir(name), 0700))
	must(t, os.WriteFile(name, []byte(body), 0600))
	f.files[rel] = body
}
func (f *fullFixture) snapshot(ctx context.Context) (*Manifest, error) {
	return Snapshot(ctx, Options{Root: f.root, Config: f.cfgFile, Output: f.output, Recipient: f.key.Recipient().String()})
}

func TestFullSnapshotRestoresDataACLAndLogin(t *testing.T) {
	f := newFullFixture(t)
	ctx := context.Background()
	// Larger than the legacy restore's 64 MiB per-file limit.
	largePath := filepath.Join(f.cfg.Mailstore.Root, "large-file.bin")
	large, err := os.Create(largePath)
	must(t, err)
	block := bytes.Repeat([]byte("large-file-test!"), 65536)
	for i := 0; i < 80; i++ {
		_, err = large.Write(block)
		must(t, err)
	}
	must(t, large.Close())
	m, err := f.snapshot(ctx)
	must(t, err)
	if m.Counts["users"] != 2 || m.Counts["xmpp_mam"] != 1 || m.Counts["tasks"] != 1 {
		t.Fatalf("missing datasets: %+v", m.Counts)
	}
	before, err := migrate.ResolveKey("", f.root)
	must(t, err)
	// Make the source inaccessible: recovery must not accidentally read it.
	must(t, os.Rename(f.root, f.root+".offline"))
	verifyTarget := filepath.Join(f.parent, "verification")
	_, err = Restore(ctx, RestoreOptions{Input: f.output, Identity: f.keyFile, Target: verifyTarget, VerifyOnly: true})
	must(t, err)
	if _, err = os.Stat(verifyTarget); !os.IsNotExist(err) {
		t.Fatal("verification published a live tree")
	}
	target := filepath.Join(f.parent, "recovered")
	_, err = Restore(ctx, RestoreOptions{Input: f.output, Identity: f.keyFile, Target: target})
	must(t, err)
	largeInfo, err := os.Stat(filepath.Join(target, "maildir", "large-file.bin"))
	must(t, err)
	if largeInfo.Size() != int64(len(block)*80) {
		t.Fatal("large file was truncated")
	}
	for rel, want := range f.files {
		got, err := os.ReadFile(filepath.Join(target, rel))
		must(t, err)
		if string(got) != want {
			t.Fatalf("lost file: %s", rel)
		}
	}
	gotCfg, err := config.Load(filepath.Join(target, "config.yaml"))
	must(t, err)
	driver, err := storage.Open(ctx, gotCfg.Storage)
	must(t, err)
	defer driver.Close()
	st := driver.(*storage.Store)
	hub := settings.NewHub(st, gotCfg)
	must(t, hub.Load(ctx))
	authn := auth.NewLayer(st, gotCfg.LDAP, gotCfg.MFA, gotCfg.OIDC)
	u, err := authn.Authenticate(ctx, "owner@example.test", "fixture-password")
	must(t, err)
	if u.ID != f.user {
		t.Fatal("user identity changed")
	}
	mb, err := st.GetMailbox(ctx, f.user, "INBOX")
	must(t, err)
	if !strings.HasPrefix(mb.Path, target+string(filepath.Separator)) {
		t.Fatal("mailbox path not relocated")
	}
	rights, err := st.MailboxRightsForUser(ctx, mb.ID, f.reader)
	must(t, err)
	if rights != "lr" {
		t.Fatalf("lost mailbox rights: %s", rights)
	}
	acl, err := st.ListFileAccess(ctx, f.user, f.reader)
	must(t, err)
	if len(acl) != 1 || acl[0].Rights != "read" {
		t.Fatalf("lost file ACL: %+v", acl)
	}
	if _, err = st.GetCalendarObjectByID(ctx, f.event); err != nil {
		t.Fatal(err)
	}
	if _, err = st.GetMessageByID(ctx, f.message); err != nil {
		t.Fatal(err)
	}
	key, err := migrate.ResolveKey("", target)
	must(t, err)
	if !bytes.Equal(key, before) {
		t.Fatal("server secret changed")
	}
	binding, err := storage.ReadDKIMBinding(ctx, st, "example.test")
	must(t, err)
	if binding.KeyFile != filepath.Join(target, "dkim/domain.pem") {
		t.Fatal("DKIM path not relocated")
	}
	if _, err = Restore(ctx, RestoreOptions{Input: f.output, Identity: f.keyFile, Target: target}); err == nil {
		t.Fatal("overwrote existing installation")
	}
}

func TestFullSnapshotRejectsUnsafeSources(t *testing.T) {
	f := newFullFixture(t)
	ctx := context.Background()
	st, err := storage.Open(ctx, f.cfg.Storage)
	must(t, err)
	if _, err = f.snapshot(ctx); err == nil {
		t.Fatal("snapshotted running installation")
	}
	must(t, st.Close())
	lock, err := maintenance.SQLite(f.cfg.Storage.SQLite.Path, true)
	must(t, err)
	if live, err := storage.Open(ctx, f.cfg.Storage); err == nil {
		live.Close()
		t.Fatal("writer opened during maintenance")
	}
	must(t, lock.Close())
	must(t, os.Symlink(f.keyFile, filepath.Join(f.root, "outside-link")))
	if _, err = f.snapshot(ctx); err == nil {
		t.Fatal("followed symlink")
	}
	must(t, os.Remove(filepath.Join(f.root, "outside-link")))
	must(t, os.Remove(filepath.Join(f.root, "maildir/owner@example.test/cur/mail.eml")))
	if _, err = f.snapshot(ctx); err == nil || !strings.Contains(err.Error(), "referenced content missing") {
		t.Fatalf("missing message ignored: %v", err)
	}
	if _, err = os.Stat(f.output); !os.IsNotExist(err) {
		t.Fatal("published incomplete backup")
	}
}

func TestFullRestoreRejectsCorruptionWrongKeyAndLimits(t *testing.T) {
	f := newFullFixture(t)
	ctx := context.Background()
	_, err := f.snapshot(ctx)
	must(t, err)
	data, err := os.ReadFile(f.output)
	must(t, err)
	wrong, err := age.GenerateX25519Identity()
	must(t, err)
	wrongFile := filepath.Join(f.parent, "wrong-key")
	must(t, os.WriteFile(wrongFile, []byte(wrong.String()), 0600))
	for _, tc := range []struct {
		name string
		data []byte
		key  string
		max  int64
	}{
		{"wrong-key", data, wrongFile, 0},
		{"truncated", data[:len(data)-1], f.keyFile, 0},
		{"size-limit", data, f.keyFile, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := filepath.Join(f.parent, tc.name+".age")
			must(t, os.WriteFile(input, tc.data, 0600))
			target := filepath.Join(f.parent, "failed-"+tc.name)
			if _, err := Restore(ctx, RestoreOptions{Input: input, Identity: tc.key, Target: target, MaxBytes: tc.max}); err == nil {
				t.Fatal("accepted invalid backup")
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatal("left failed installation")
			}
		})
	}
	// A valid encryption envelope is not sufficient: reject hostile tar names.
	for _, name := range []string{"data/../escape", "data//absolute", "data/C:/escape"} {
		var b bytes.Buffer
		enc, err := age.Encrypt(&b, f.key.Recipient())
		must(t, err)
		gz := gzip.NewWriter(enc)
		tw := tar.NewWriter(gz)
		must(t, writeJSON(tw, "header.json", Header{Format: format, Root: f.root, Config: "config.yaml", Database: "tayga.db"}))
		must(t, tw.WriteHeader(&tar.Header{Name: name, Size: 1, Mode: 0600}))
		_, err = io.WriteString(tw, "x")
		must(t, err)
		must(t, tw.Close())
		must(t, gz.Close())
		must(t, enc.Close())
		input := filepath.Join(f.parent, "malicious.age")
		must(t, os.WriteFile(input, b.Bytes(), 0600))
		if _, err = Restore(ctx, RestoreOptions{Input: input, Identity: f.keyFile, Target: filepath.Join(f.parent, "escape-test")}); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
}
