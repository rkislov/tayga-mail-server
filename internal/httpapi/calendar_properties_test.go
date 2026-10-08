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

func TestCalendarPropertiesSharingAndRevocation(t *testing.T) {
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
	c, err := st.EnsureCalendar(ctx, users["owner"].ID, "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/calendar/calendars/" + c.ID
	call("owner", "PATCH", base, `{"display_name":"Рабочий","color":"#3388ff","description":"Plans"}`, 200)
	saved, err := st.GetCalendarByID(ctx, users["owner"].ID, c.ID)
	if err != nil || saved.DisplayName != "Рабочий" || saved.Color != "#3388ff" {
		t.Fatalf("properties not saved: %+v %v", saved, err)
	}
	call("owner", "PATCH", base, `{"color":"red"}`, 400)
	empty := call("owner", "PATCH", base, `{"public_enabled":true}`, 200)
	var ep map[string]any
	json.Unmarshal(empty.Body.Bytes(), &ep)
	eu := ep["public_url"].(string)
	call("", "GET", eu[strings.Index(eu, "/calendar/public/"):], "", 200)
	call("stranger", "PATCH", base, `{"display_name":"stolen"}`, 404)
	call("owner", "PUT", base+"/acl", `{"email":"guest@ex.com","rights":"admin"}`, 400)
	call("owner", "PUT", base+"/acl", `{"email":"guest@ex.com","rights":"read"}`, 200)
	call("guest", "GET", base, "", 200)
	call("guest", "PATCH", base, `{"color":"#ffffff"}`, 403)
	call("guest", "POST", base+"/events", `{"summary":"Denied"}`, 404)
	call("owner", "PUT", base+"/acl", `{"email":"guest@ex.com","rights":"write"}`, 200)
	w := call("guest", "POST", base+"/events", `{"summary":"Shared meeting","start":"2026-10-10T10:00:00Z","end":"2026-10-10T11:00:00Z"}`, 200)
	var event map[string]any
	json.Unmarshal(w.Body.Bytes(), &event)
	call("guest", "PUT", base+"/events", `{"id":"`+event["id"].(string)+`","summary":"Edited meeting","start":"2026-10-10T10:00:00Z","end":"2026-10-10T11:00:00Z"}`, 200)
	// An unrelated event id must not be usable to overwrite/move another user's event.
	other, _ := st.EnsureCalendar(ctx, users["stranger"].ID, "default", "Private")
	private, _ := st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: other.ID, UID: "private", HrefName: "private.ics", Data: "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n", Component: "VEVENT"})
	call("guest", "POST", base+"/events", `{"id":"`+private.ID+`","summary":"Overwrite"}`, 403)
	w = call("owner", "PATCH", base, `{"public_enabled":true}`, 200)
	var props map[string]any
	json.Unmarshal(w.Body.Bytes(), &props)
	url := props["public_url"].(string)
	path := url[strings.Index(url, "/calendar/public/"):]
	public := call("", "GET", path, "", 200)
	if !strings.Contains(public.Body.String(), "Edited meeting") || strings.Count(public.Body.String(), "BEGIN:VCALENDAR") != 1 {
		t.Fatal("invalid calendar export")
	}
	guest := call("guest", "GET", base, "", 200)
	if strings.Contains(guest.Body.String(), "public_url") {
		t.Fatal("public management secret exposed to grantee")
	}
	call("", "POST", path, "", 405)
	call("owner", "PATCH", base, `{"public_enabled":true,"rotate_link":true}`, 200)
	call("", "GET", path, "", 404)
	refreshed, _ := st.GetCalendarByID(ctx, users["owner"].ID, c.ID)
	newPath := "/calendar/public/" + refreshed.PublicToken + ".ics"
	call("", "GET", newPath, "", 200)
	call("owner", "PATCH", base, `{"public_enabled":false}`, 200)
	call("", "GET", newPath, "", 404)
	call("guest", "DELETE", "/api/v1/calendar/events/"+event["id"].(string), "", 200)
	call("owner", "DELETE", base+"/acl/"+users["guest"].ID, "", 200)
	call("guest", "GET", base+"/events", "", 404)
	call("owner", "DELETE", base, "", 400)
}
