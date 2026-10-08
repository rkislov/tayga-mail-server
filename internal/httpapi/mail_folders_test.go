package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestMailFolderManagementAndArchive(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := config.Default()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	layer := auth.NewLayer(st, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("secret")
	tenant, _ := st.CreateTenant(ctx, "t")
	dom, _ := st.CreateDomain(ctx, tenant.ID, "ex.com")
	u, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: dom.ID, Email: "u@ex.com", LocalPart: "u", Enabled: true, AuthSource: "local", PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	ms := mailstore.New(filepath.Join(dir, "mail"))
	h := httpapi.New(cfg, slog.Default(), st, layer, ms, nil, nil).Handler()
	login, err := layer.LoginWithPassword(ctx, u.Email, "secret")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string, code int) []byte {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+login.Tokens.AccessToken)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != code {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	call("GET", "/api/v1/mail/mailboxes", "", 200)
	for _, name := range []string{"INBOX", "Sent", "Drafts", "Trash", "Junk", "Archive"} {
		mb, _ := st.GetMailbox(ctx, u.ID, name)
		call("DELETE", "/api/v1/mail/mailboxes/"+mb.ID, "", 403)
		call("PATCH", "/api/v1/mail/mailboxes/"+mb.ID, `{"name":"Renamed"}`, 403)
	}
	call("POST", "/api/v1/mail/mailboxes", `{"name":"Projects"}`, 201)
	call("POST", "/api/v1/mail/mailboxes", `{"name":"../escape"}`, 400)
	custom, _ := st.GetMailbox(ctx, u.ID, "Projects")
	raw := []byte("Subject: Custom message\r\nFrom: a@ex.com\r\n\r\nbody")
	rel, size, err := ms.Deliver(u.Email, "Projects", raw)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: custom.ID, FilePath: rel, Size: size})
	if err != nil {
		t.Fatal(err)
	}
	call("PATCH", "/api/v1/mail/mailboxes/"+custom.ID, `{"name":"Work"}`, 200)
	renamed, _ := st.GetMessageByID(ctx, msg.ID)
	if data, e := ms.Read(renamed.FilePath); e != nil || !bytes.Equal(data, raw) {
		t.Fatalf("renamed message missing: %v", e)
	}
	inbox, _ := st.GetMailbox(ctx, u.ID, "INBOX")
	order, _ := json.Marshal(map[string]any{"order": []string{inbox.ID, custom.ID}})
	call("PUT", "/api/v1/mail/mailboxes/order", string(order), 200)
	saved := call("GET", "/api/v1/mail/mailboxes", "", 200)
	if !bytes.Contains(saved, order[1:len(order)-1]) {
		t.Fatalf("order not persisted %s", saved)
	}
	order, _ = json.Marshal(map[string]any{"order": []string{custom.ID, inbox.ID}})
	call("PUT", "/api/v1/mail/mailboxes/order", string(order), 400)
	archive, _ := st.GetMailbox(ctx, u.ID, "Archive")
	rel, size, err = ms.Deliver(u.Email, "Archive", raw)
	if err != nil {
		t.Fatal(err)
	}
	archived, err := st.InsertMessage(ctx, &storage.Message{MailboxID: archive.ID, FilePath: rel, Size: size, Archived: true})
	if err != nil {
		t.Fatal(err)
	}
	list := call(http.MethodGet, "/api/v1/mail/mailboxes/"+archive.ID+"/messages", "", 200)
	if !bytes.Contains(list, []byte(archived.ID)) {
		t.Fatal("archive message not listed")
	}
	call("GET", "/api/v1/mail/messages/"+archived.ID, "", 200)
	call("DELETE", "/api/v1/mail/mailboxes/"+custom.ID, "", 200)
	if _, err = st.GetMessageByID(ctx, msg.ID); err != storage.ErrNotFound {
		t.Fatalf("deleted folder message: %v", err)
	}
}
