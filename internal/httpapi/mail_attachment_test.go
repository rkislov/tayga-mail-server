package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestMailAttachmentOwnershipAndDownload(t *testing.T) {
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
	inbox, _ := st.GetMailbox(ctx, u.ID, "INBOX")
	raw := []byte("Content-Type: multipart/mixed; boundary=x\r\n\r\n--x\r\nContent-Type: text/plain\r\n\r\nbody\r\n--x\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=ticket.pdf\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi10ZXN0\r\n--x--\r\n")
	rel, size, err := ms.Deliver(u.Email, "INBOX", raw)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: inbox.ID, FilePath: rel, Size: size})
	if err != nil {
		t.Fatal(err)
	}
	travelRaw := []byte("Content-Type: text/plain; charset=utf-8\r\n\r\nРейс: SU1234\nВылет: 09.10.2026 12:30\nМесто: 12A\n")
	travelPath, travelSize, _ := ms.Deliver(u.Email, "INBOX", travelRaw)
	travelMsg, _ := st.InsertMessage(ctx, &storage.Message{MailboxID: inbox.ID, FilePath: travelPath, Size: travelSize})
	gotTrip := call("GET", "/api/v1/mail/messages/"+travelMsg.ID+"/travel", "", 200)
	if !bytes.Contains(gotTrip, []byte("SU1234")) || !bytes.Contains(gotTrip, []byte("12A")) {
		t.Fatal("trip extraction missing", string(gotTrip))
	}
	if err := st.DeleteMessage(ctx, travelMsg.ID); err != nil {
		t.Fatal(err)
	}
	call("PUT", "/api/v1/mail/image-preferences", `{"mode":"block","senders":["Sender <sender@EX.com>"]}`, 200)
	if got := call("GET", "/api/v1/mail/image-preferences", "", 200); !bytes.Contains(got, []byte("sender@ex.com")) {
		t.Fatal("sender preference not saved")
	}
	call("PUT", "/api/v1/mail/image-preferences", `{"mode":"invalid"}`, 400)
	for i := 0; i < 4; i++ {
		_, err := st.InsertMessage(ctx, &storage.Message{MailboxID: inbox.ID, FilePath: rel, Size: size, Subject: "Ticket", FromAddr: "sender@ex.com"})
		if err != nil {
			t.Fatal(err)
		}
	}
	var first, second struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
		Total   int  `json:"total"`
		HasMore bool `json:"has_more"`
	}
	json.Unmarshal(call("GET", "/api/v1/mail/mailboxes/"+inbox.ID+"/messages?limit=2&offset=0", "", 200), &first)
	json.Unmarshal(call("GET", "/api/v1/mail/mailboxes/"+inbox.ID+"/messages?limit=2&offset=2", "", 200), &second)
	if first.Total != 5 || len(first.Messages) != 2 || len(second.Messages) != 2 || first.Messages[0].ID == second.Messages[0].ID {
		t.Fatal("mailbox pagination repeated or missing messages")
	}
	first.Messages = nil
	second.Messages = nil
	json.Unmarshal(call("GET", "/api/v1/mail/search?q=subject:Ticket&limit=2&offset=0", "", 200), &first)
	json.Unmarshal(call("GET", "/api/v1/mail/search?q=subject:Ticket&limit=2&offset=2", "", 200), &second)
	if !first.HasMore || second.HasMore || len(first.Messages) != 2 || len(second.Messages) != 2 || first.Messages[0].ID == second.Messages[0].ID {
		t.Fatal("search pagination repeated or missing messages")
	}
	base := "/api/v1/mail/messages/" + msg.ID + "/attachments/"
	if got := call("GET", base+"0", "", 200); string(got) != "%PDF-test" {
		t.Fatalf("corrupt downloaded attachment %q", got)
	}
	call("GET", base+"99", "", 404)
	st.UpdateMessageFlags(ctx, msg.ID, `\Seen \Flagged`)
	var filtered struct {
		Total int `json:"total"`
	}
	json.Unmarshal(call("GET", "/api/v1/mail/mailboxes/"+inbox.ID+"/messages?filter=unread", "", 200), &filtered)
	if filtered.Total != 4 {
		t.Fatalf("unread filter: %d", filtered.Total)
	}
	json.Unmarshal(call("GET", "/api/v1/mail/mailboxes/"+inbox.ID+"/messages?filter=flagged", "", 200), &filtered)
	if filtered.Total != 1 {
		t.Fatalf("flagged filter: %d", filtered.Total)
	}
	if !bytes.Contains(call("GET", "/api/v1/mail/mailboxes/"+inbox.ID+"/messages", "", 200), []byte(`"has_attachments":true`)) {
		t.Fatal("missing attachment indicator")
	}
	r := httptest.NewRequest("GET", base+"0", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("anonymous download %d", w.Code)
	}
	other, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: dom.ID, Email: "other@ex.com", LocalPart: "other", Enabled: true, AuthSource: "local", PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	otherLogin, err := layer.LoginWithPassword(ctx, other.Email, "secret")
	if err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRequest("GET", base+"0", nil)
	r.Header.Set("Authorization", "Bearer "+otherLogin.Tokens.AccessToken)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	rPrefs := httptest.NewRequest("GET", "/api/v1/mail/image-preferences", nil)
	rPrefs.Header.Set("Authorization", "Bearer "+otherLogin.Tokens.AccessToken)
	wPrefs := httptest.NewRecorder()
	h.ServeHTTP(wPrefs, rPrefs)
	if wPrefs.Code != 200 || bytes.Contains(wPrefs.Body.Bytes(), []byte("sender@ex.com")) || !bytes.Contains(wPrefs.Body.Bytes(), []byte("block")) {
		t.Fatal("image preferences leaked between accounts")
	}
	if w.Code == 200 || strings.Contains(w.Body.String(), "%PDF-test") {
		t.Fatalf("cross-account attachment access %d", w.Code)
	}
}
