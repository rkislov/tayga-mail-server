package mmplugins

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

type smokePlugin struct {
	plugin.MattermostPlugin
	message string
}

func (p *smokePlugin) load() error {
	var config struct{ Message string }
	if err := p.API.LoadPluginConfiguration(&config); err != nil {
		return err
	}
	p.message = config.Message
	return nil
}
func (p *smokePlugin) OnActivate() error {
	if err := p.load(); err != nil {
		return err
	}
	if err := p.API.KVSet("activation", []byte("initial")); err != nil {
		return err
	}
	ok, appErr := p.API.KVCompareAndSet("activation", []byte("initial"), []byte("ready"))
	if appErr != nil {
		return appErr
	}
	if !ok {
		return fmt.Errorf("KV CAS failed")
	}
	if _, e := p.API.GetUser("not-a-user"); e == nil {
		return fmt.Errorf("missing user unexpectedly found")
	}
	return p.API.RegisterCommand(&model.Command{Trigger: "smoke", AutoComplete: true})
}
func (p *smokePlugin) OnConfigurationChange() error { return p.load() }
func (p *smokePlugin) OnDeactivate() error {
	if e := p.API.KVSet("deactivated", []byte("yes")); e != nil {
		return e
	}
	return nil
}
func (p *smokePlugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	data, err := p.API.KVGet("activation")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"message": p.message, "activation": string(data), "user_id": r.Header.Get("Mattermost-User-Id"), "auth": r.Header.Get("Authorization"), "cookie": r.Header.Get("Cookie"), "path": r.URL.Path})
}
func (p *smokePlugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: p.message}, nil
}
func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == "tms-plugin-smoke" {
		plugin.ClientMain(&smokePlugin{})
		return
	}
	os.Exit(m.Run())
}

func testOptions(t *testing.T) Options {
	t.Helper()
	cfg := config.Default()
	cfg.Storage.SQLite.Path = filepath.Join(t.TempDir(), "plugins.db")
	st, err := storage.Open(context.Background(), cfg.Storage)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return Options{Directory: filepath.Join(t.TempDir(), "plugins"), SiteURL: "https://mail.example.test", Store: st, Chat: chat.New(st), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}
func pluginArchive(t *testing.T, manifest map[string]any, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	all := map[string][]byte{"plugin.json": raw}
	for k, v := range files {
		all[k] = v
	}
	for name, content := range all {
		if err = tw.WriteHeader(&tar.Header{Name: "bundle/" + name, Mode: 0700, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err = tw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err = tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
func TestRealSDKPluginLifecycle(t *testing.T) {
	options := testOptions(t)
	runtime, err := New(options)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	archive := pluginArchive(t, map[string]any{"id": "com.tms.smoke", "name": "Smoke", "version": "1.0.0", "min_server_version": "9.11.0", "server": map[string]any{"executable": "server/tms-plugin-smoke"}, "settings_schema": map[string]any{"settings": []any{map[string]any{"key": "Message", "type": "text", "default": "hello"}}}}, map[string][]byte{"server/tms-plugin-smoke": payload})
	manifest, err := runtime.Install(bytes.NewReader(archive), false)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Id != "com.tms.smoke" {
		t.Fatal(manifest)
	}
	if _, err = runtime.Enable(manifest.Id); err != nil {
		t.Fatal(err)
	}
	if len(runtime.Commands("")) != 1 {
		t.Fatal("command not registered")
	}
	args := &model.CommandArgs{Command: "/smoke", UserId: chat.ID("user")}
	result, appErr := runtime.Execute(args)
	if appErr != nil || result.Text != "hello" {
		t.Fatalf("execute: %v %v", result, appErr)
	}
	req := httptest.NewRequest("GET", "/plugins/com.tms.smoke/echo", nil)
	req.Header.Set("Mattermost-User-Id", "spoofed")
	req.Header.Set("Authorization", "Bearer private")
	req.Header.Set("Cookie", "private=value")
	w := httptest.NewRecorder()
	runtime.ServeHTTP(w, req, "")
	var httpResult map[string]string
	if err = json.Unmarshal(w.Body.Bytes(), &httpResult); err != nil {
		t.Fatalf("http: %d %s %v", w.Code, w.Body, err)
	}
	if httpResult["activation"] != "ready" || httpResult["user_id"] != "" || httpResult["auth"] != "" || httpResult["cookie"] != "" || httpResult["path"] != "/echo" {
		t.Fatal(httpResult)
	}
	if err = runtime.Configure(manifest.Id, map[string]any{"Message": "configured"}); err != nil {
		t.Fatal(err)
	}
	result, appErr = runtime.Execute(args)
	if appErr != nil || result.Text != "configured" {
		t.Fatalf("configured execute: %v %v", result, appErr)
	}
	runtime.Close()
	runtime, err = New(options)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	result, appErr = runtime.Execute(args)
	if appErr != nil || result.Text != "configured" {
		t.Fatalf("restart: %v %v", result, appErr)
	}
	if err = runtime.Disable(manifest.Id); err != nil {
		t.Fatal(err)
	}
	if len(runtime.Commands("")) != 0 {
		t.Fatal("disabled command remains")
	}
	if _, err = runtime.Enable(manifest.Id); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.Install(bytes.NewReader(archive), true); err == nil {
		t.Fatal("active binary replaced")
	}
	if err = runtime.Remove(manifest.Id); err != nil {
		t.Fatal(err)
	}
	all, err := runtime.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(all["active"].([]*model.Manifest)) != 0 || len(all["inactive"].([]*model.Manifest)) != 0 {
		t.Fatal(all)
	}
}
func TestPluginArchiveTraversalRejected(t *testing.T) {
	options := testOptions(t)
	r, err := New(options)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	archive := pluginArchive(t, map[string]any{"id": "com.tms.bad", "name": "Bad", "version": "1.0.0"}, map[string][]byte{"../../escaped": []byte("bad")})
	if _, err = r.Install(bytes.NewReader(archive), false); err == nil {
		t.Fatal("path traversal accepted")
	}
	if _, err = os.Stat(filepath.Join(options.Directory, "escaped")); !os.IsNotExist(err) {
		t.Fatalf("escaped file exists: %v", err)
	}
}

func TestScopedPluginStorageDoesNotMixTokens(t *testing.T) {
	options := testOptions(t)
	first := &hostAPI{unsupportedAPI: &unsupportedAPI{log: options.Logger, pluginID: "github"}, manifest: &model.Manifest{Id: "github"}, runtime: &Runtime{options: options}}
	first.runtime.options.Scope = "tenant-a:domain-a"
	second := &hostAPI{unsupportedAPI: &unsupportedAPI{log: options.Logger, pluginID: "github"}, manifest: &model.Manifest{Id: "github"}, runtime: &Runtime{options: options}}
	second.runtime.options.Scope = "tenant-b:domain-b"
	if err := first.KVSet("oauth_token", []byte("private-a")); err != nil {
		t.Fatal(err)
	}
	if err := first.SavePluginConfig(map[string]any{"ClientSecret": "secret-a"}); err != nil {
		t.Fatal(err)
	}
	value, err := second.KVGet("oauth_token")
	if err != nil || value != nil {
		t.Fatalf("token leaked: %s %v", value, err)
	}
	if len(second.GetPluginConfig()) != 0 {
		t.Fatal("plugin config leaked")
	}
	if err := second.KVSet("oauth_token", []byte("private-b")); err != nil {
		t.Fatal(err)
	}
	original, err := first.KVGet("oauth_token")
	if err != nil || string(original) != "private-a" {
		t.Fatalf("tenant value changed: %s %v", original, err)
	}
}
