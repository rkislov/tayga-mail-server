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
	entries := []map[string]string{}
	for _, id := range []string{"focalboard", "playbooks"} {
		name := mmplugins.ProductDatabaseName(scope.Key(), id)
		password := strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err = admin.Exec(`CREATE ROLE "` + name + `" LOGIN PASSWORD '` + password + `' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS`); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = admin.Exec(`DROP ROLE "` + name + `"`) }()
		if _, err = admin.Exec(`CREATE DATABASE "` + name + `" OWNER "` + name + `"`); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = admin.Exec(`DROP DATABASE "` + name + `" WITH (FORCE)`) }()
		target := *parsed
		target.Path = "/" + name
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
	stop := make(chan struct{})
	mux := http.NewServeMux()
	mux.Handle("/api/v4/", f.api)
	mux.Handle("/assets/", frontend.Handler())
	mux.HandleFunc("/mm/", f.api.ServeScopedPluginHTTP)
	mux.HandleFunc("/test-login", func(w http.ResponseWriter, r *http.Request) {
		value, _ := json.Marshal(map[string]string{"access_token": f.tokens[0]})
		product := "playbooks"
		if r.URL.Query().Get("product") == "boards" {
			product = "boards"
		}
		url := "/mm/" + chat.ID(scope.Key()) + "/" + product + "?lang=" + productLocale(r)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<script>localStorage.setItem('tayga.tokens',%q);location.replace(%q)</script>`, string(value), url)
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
