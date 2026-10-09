package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoteBodyEditsReplaceDocumentAndPreserveStructure(t *testing.T) {
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
	ten, _ := st.CreateTenant(ctx, "t")
	dom, _ := st.CreateDomain(ctx, ten.ID, "ex.com")
	tokens := map[string]string{}
	users := map[string]*storage.User{}
	for _, name := range []string{"owner", "guest", "stranger"} {
		u, e := st.CreateUser(ctx, &storage.User{TenantID: ten.ID, DomainID: dom.ID, Email: name + "@ex.com", LocalPart: name, Enabled: true, AuthSource: "local", PasswordHash: hash})
		if e != nil {
			t.Fatal(e)
		}
		users[name] = u
		l, e := layer.LoginWithPassword(ctx, u.Email, "secret")
		if e != nil {
			t.Fatal(e)
		}
		tokens[name] = l.Tokens.AccessToken
	}
	handler := httpapi.New(cfg, slog.Default(), st, layer, mailstore.New(filepath.Join(dir, "mail")), nil, nil).Handler()
	call := func(user, method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if user != "" {
			r.Header.Set("Authorization", "Bearer "+tokens[user])
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s as %s: %d %s", method, path, user, w.Code, w.Body.String())
		}
		return w
	}

	w := call("owner", "POST", "/api/v1/notes", `{"title":"Draft","body_html":"<p>Original</p>"}`, 201)
	var note map[string]any
	json.Unmarshal(w.Body.Bytes(), &note)
	base := "/api/v1/notes/" + note["id"].(string)
	body := `<p>Introduction</p><ul data-type="checklist"><li data-checked="true"><input type="checkbox" checked><div data-task-text>Done</div></li></ul><ol><li>First</li></ol><p>Conclusion</p>`
	payload, _ := json.Marshal(map[string]any{"body_html": body, "etag": note["etag"]})
	w = call("owner", "PATCH", base, string(payload), 200)
	json.Unmarshal(w.Body.Bytes(), &note)
	html := note["body_html"].(string)
	if !strings.Contains(html, "Introduction") || !strings.Contains(html, "Conclusion") || !strings.Contains(html, `data-checked="true"`) || !strings.Contains(html, "<ol>") {
		t.Fatalf("body edit discarded or flattened: %s", html)
	}
	payload, _ = json.Marshal(map[string]any{"title": "Renamed", "etag": note["etag"]})
	w = call("owner", "PATCH", base, string(payload), 200)
	json.Unmarshal(w.Body.Bytes(), &note)
	if note["body_html"] != html {
		t.Fatal("title patch changed document")
	}
	call("owner", "PATCH", base, `{"etag":"stale","body_html":"<p>overwrite</p>"}`, 409)
	w = call("owner", "PATCH", base, `{"body_html":""}`, 200)
	json.Unmarshal(w.Body.Bytes(), &note)
	if note["body_text"] != "" {
		t.Fatal("cannot clear note")
	}
	call("stranger", "PATCH", base, `{"body_html":"stolen"}`, 404)
}
