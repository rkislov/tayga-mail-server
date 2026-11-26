package mattermost

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/frontend"
	"github.com/tayga/tms/internal/mmplugins"
)

// Opt-in local browser fixture. Uses only synthetic identities and disposable
// databases. /test-login is confined to loopback and is never a production route.
func TestProductBrowserStand(t *testing.T) {
	dsn := os.Getenv("TMS_TEST_PLUGIN_POSTGRES")
	bundles := os.Getenv("TMS_TEST_PRODUCT_BROWSER_BUNDLES")
	if dsn == "" || bundles == "" {
		t.Skip("optional loopback browser stand")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Hostname() != "127.0.0.1" {
		t.Fatal("localhost PostgreSQL required")
	}
	f := setup(t)
	f.api.log = slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()
	if err = f.store.UpdateUserProfile(ctx, f.users[0].ID, "Алиса Тестовая"); err != nil {
		t.Fatal(err)
	}
	if err = f.store.UpdateUserProfile(ctx, f.users[1].ID, ""); err != nil {
		t.Fatal(err)
	}
	scope := userScope(f.users[0])
	directory := filepath.Join(t.TempDir(), "plugins")
	if err = os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	database := mmplugins.SharedProductDatabaseName(scope.Key())
	if _, err = admin.Exec(`CREATE DATABASE "` + database + `" TEMPLATE template0`); err != nil {
		t.Fatal(err)
	}
	dropRoles := []string{}
	defer func() {
		_, _ = admin.Exec(`DROP DATABASE "` + database + `" WITH (FORCE)`)
		for _, role := range dropRoles {
			_, _ = admin.Exec(`DROP ROLE "` + role + `"`)
		}
	}()
	if _, err = admin.Exec(`REVOKE ALL ON DATABASE "` + database + `" FROM PUBLIC`); err != nil {
		t.Fatal(err)
	}
	sharedDSN := *parsed
	sharedDSN.Path = "/" + database
	shared, err := sql.Open("postgres", sharedDSN.String())
	if err != nil {
		t.Fatal(err)
	}
	defer shared.Close()
	if _, err = shared.Exec(`REVOKE ALL ON SCHEMA public FROM PUBLIC`); err != nil {
		t.Fatal(err)
	}
	entries := []map[string]string{}
	for _, id := range []string{"focalboard", "playbooks"} {
		name := mmplugins.ProductDatabaseName(scope.Key(), id)
		password := strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err = admin.Exec(`CREATE ROLE "` + name + `" LOGIN PASSWORD '` + password + `' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS`); err != nil {
			t.Fatal(err)
		}
		dropRoles = append(dropRoles, name)
		schema := mmplugins.ProductDatabaseSchema(database, scope.Key(), id)
		schemaDDL := `ALTER SCHEMA public OWNER TO "` + name + `"`
		if schema == "boards" {
			schemaDDL = `CREATE SCHEMA boards AUTHORIZATION "` + name + `"`
		}
		if _, err = shared.Exec(schemaDDL + `; GRANT CONNECT,TEMPORARY ON DATABASE "` + database + `" TO "` + name + `";
			ALTER ROLE "` + name + `" IN DATABASE "` + database + `" SET search_path TO ` + schema); err != nil {
			t.Fatal(err)
		}
		target := *parsed
		target.Path = "/" + database
		target.User = url.UserPassword(name, password)
		entries = append(entries, map[string]string{"scope": scope.Key(), "plugin_id": id, "dsn": target.String()})
	}
	raw, _ := json.Marshal(entries)
	if err = os.WriteFile(filepath.Join(directory, ".databases.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:59864")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	f.api.siteURL = "http://" + listener.Addr().String()
	if err = f.api.EnablePlugins(directory, nil); err != nil {
		t.Fatal(err)
	}
	defer f.api.Close()
	for _, product := range []struct{ id, file string }{{"focalboard", fmt.Sprintf("mattermost-plugin-boards-v10.1.0-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)}, {"playbooks", fmt.Sprintf("tayga-playbooks-2.9.7-tms.1-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)}} {
		file, err := os.Open(filepath.Join(bundles, product.file))
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.api.plugins.Install(file, false)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err = f.api.policy.SetPluginEnabled(ctx, scope, product.id, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = f.api.pluginRuntimeForScope(ctx, scope); err != nil {
		t.Fatal(err)
	}
	base := "/mm/" + chat.ID(scope.Key())
	create := func(user int, collection, body string) string {
		r := httptest.NewRequest("POST", base+"/plugins/playbooks/api/v0/"+collection, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+f.tokens[user])
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		f.api.ServeScopedPluginHTTP(w, r)
		var created struct{ ID string }
		if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &created) != nil || created.ID == "" {
			t.Fatalf("create browser %s fixture: %d %s", collection, w.Code, w.Body.String())
		}
		return created.ID
	}
	team := chat.ID(scope.TenantID)
	tutorial := create(0, "playbooks", fmt.Sprintf(`{"title":"Learn how to use playbooks","team_id":%q,"public":true,"status_update_enabled":true,"reminder_timer_default_seconds":3600,"checklists":[{"title":"Checklist","items":[{"title":"Read the procedure"}]}]}`, team))
	legacyRuns := map[int]string{}
	for user, name := range []string{"alice", "bob"} {
		legacyRuns[user] = create(user, "runs", fmt.Sprintf(`{"name":%q,"summary":"Contrast sample","owner_user_id":%q,"team_id":%q,"playbook_id":%q}`, name+"'s onboarding run", chat.ID(f.users[user].ID), team, tutorial))
	}
	stop := make(chan struct{})
	mux := http.NewServeMux()
	mux.Handle("/api/v4/", f.api)
	mux.Handle("/assets/", frontend.Handler())
	mux.HandleFunc("/mm/", f.api.ServeScopedPluginHTTP)
	mux.HandleFunc("/test-login", func(w http.ResponseWriter, r *http.Request) {
		user := 0
		if r.URL.Query().Get("profile") == "account-only" {
			user = 1
		}
		value, _ := json.Marshal(map[string]string{"access_token": f.tokens[user]})
		product := "playbooks"
		if r.URL.Query().Get("product") == "boards" {
			product = "boards"
		}
		path := base + "/" + product
		if product == "playbooks" && r.URL.Query().Get("run") == "legacy" {
			path += "/runs/" + legacyRuns[user]
		}
		url := path + "?lang=" + productLocale(r)
		theme := "light"
		if r.URL.Query().Get("theme") == "dark" {
			theme = "dark"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<script>localStorage.setItem('tayga.tokens',%q);localStorage.setItem('tayga.colorMode',%q);location.replace(%q)</script>`, string(value), theme, url)
	})
	mux.HandleFunc("/test-stop", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-stop:
		default:
			close(stop)
		}
		w.WriteHeader(204)
	})
	logging := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "websocket") {
			mux.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		for name, values := range rec.Header() {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		w.WriteHeader(rec.Code)
		w.Write(rec.Body.Bytes())
		if rec.Code >= 400 {
			body := rec.Body.String()
			if len(body) > 4096 {
				body = body[:4096]
			}
			t.Log("fixture HTTP error", r.Method, r.URL.Path, rec.Code, body)
		}
	})
	server := &http.Server{Handler: logging, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go server.Serve(listener)
	t.Log("Synthetic browser fixture ready: http://127.0.0.1:59864/test-login")
	select {
	case <-stop:
	case <-time.After(30 * time.Minute):
		t.Fatal("browser fixture expired")
	}
}
