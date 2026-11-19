package mmplugins

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

//go:embed testdata/marketplace.json
var marketplacePins []byte

type marketplaceBundle struct {
	Name, ID, Version string
	Assets            map[string]struct{ URL, SHA256 string }
}

// Real marketplace bundles are optional: regular tests never download or execute
// unpinned third-party code. See docs/mattermost-plugins.md for acquisition.
func TestMarketplacePluginBundles(t *testing.T) {
	dir := os.Getenv("TMS_TEST_PLUGIN_BUNDLES")
	if dir == "" {
		t.Skip("set TMS_TEST_PLUGIN_BUNDLES to the verified bundle directory")
	}
	var bundles []marketplaceBundle
	if err := json.Unmarshal(marketplacePins, &bundles); err != nil {
		t.Fatal(err)
	}
	for _, bundle := range bundles {
		t.Run(bundle.Name, func(t *testing.T) {
			name := bundle.Name
			asset, ok := bundle.Assets[runtime.GOOS+"-"+runtime.GOARCH]
			if !ok {
				t.Skip("no pinned native bundle for this platform")
			}

			opts := testOptions(t)
			var logs bytes.Buffer
			opts.Logger = slog.New(slog.NewTextHandler(&logs, nil))
			t.Cleanup(func() { t.Log(logs.String()) })
			ctx := context.Background()
			ten, err := opts.Store.CreateTenant(ctx, "Marketplace acceptance")
			if err != nil {
				t.Fatal(err)
			}
			dom, err := opts.Store.CreateDomain(ctx, ten.ID, "example.test")
			if err != nil {
				t.Fatal(err)
			}
			u, err := opts.Store.CreateUser(ctx, &storage.User{TenantID: ten.ID, DomainID: dom.ID, Email: "owner@example.test", LocalPart: "owner", DisplayName: "Fixture Owner", Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			opts.Scope = "domain:" + ten.ID + ":" + dom.ID
			opts.TenantID, opts.DomainID = ten.ID, dom.ID
			stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unconfigured test upstream", http.StatusServiceUnavailable)
			}))
			defer stub.Close()
			r, err := New(opts)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			f, err := os.Open(filepath.Join(dir, name+".tar.gz"))
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.New()
			count, err := io.Copy(hash, io.LimitReader(f, (100<<20)+1))
			if err != nil || count > 100<<20 || hex.EncodeToString(hash.Sum(nil)) != asset.SHA256 {
				f.Close()
				t.Fatal("bundle checksum or size verification failed", err)
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				f.Close()
				t.Fatal(err)
			}
			manifest, err := r.Install(f, false)
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			if manifest.Id != bundle.ID || manifest.Version != bundle.Version {
				t.Fatal("bundle manifest differs from pinned release")
			}
			t.Logf("installed %s %s, minimum server %s", manifest.Id, manifest.Version, manifest.MinServerVersion)
			config := map[string]any{"GitlabURL": stub.URL, "GitlabOAuthClientID": "fixture", "GitlabOAuthClientSecret": "fixture", "GitHubEnterpriseBaseURL": stub.URL, "GitHubEnterpriseUploadURL": stub.URL, "GitHubOAuthClientID": "fixture", "GitHubOAuthClientSecret": "fixture", "WebhookSecret": strings.Repeat("fixture", 8), "EncryptionKey": strings.Repeat("a", 32), "UsePreregisteredApplication": false, "BASE_URL": stub.URL + "/bigbluebutton/api", "SALT": strings.Repeat("fixture", 8), "PROCESS_RECORDINGS": false}
			if err := r.Configure(manifest.Id, config); err != nil {
				t.Fatal(err)
			}
			if _, err := r.Enable(manifest.Id); err != nil {
				t.Fatalf("activation: %v", err)
			}
			trigger := name
			if name == "bigbluebutton" {
				trigger = "bbb"
			}
			if name == "github3" {
				trigger = "github"
			}
			if err := opts.Chat.EnsureDefault(ctx, u); err != nil {
				t.Fatal(err)
			}
			channels, err := opts.Chat.Channels(ctx, u)
			if err != nil || len(channels) == 0 {
				t.Fatal("missing fixture channel", err)
			}
			channel := channels[0]
			events, unsubscribe := opts.Chat.Subscribe()
			defer unsubscribe()
			command := "/" + trigger + " help"
			if name == "bigbluebutton" {
				command = "/bbb"
			}
			response, appErr := r.Execute(&model.CommandArgs{Command: command, UserId: chat.ID(u.ID), TeamId: chat.ID(u.TenantID), ChannelId: channel.ID})
			if appErr != nil {
				t.Fatalf("help command: %v", appErr)
			}
			if name == "bigbluebutton" {
				select {
				case ev := <-events:
					saved, err := opts.Chat.Post(ctx, u, ev.Post.ID)
					if err != nil || saved.ChannelID != channel.ID || saved.Props["attachments"] == nil {
						t.Fatal("missing persisted BBB invitation card", err)
					}
				case <-time.After(time.Second):
					t.Fatal("missing BBB invitation card")
				}
				t.Log("activation and /bbb invitation card passed; media service was not contacted")
			} else if response == nil || response.Text == "" {
				select {
				case event := <-events:
					var post model.Post
					raw, _ := event.Data["post"].(string)
					if event.Type != "ephemeral_message" || event.UserID != chat.ID(u.ID) || json.Unmarshal([]byte(raw), &post) != nil || post.Message == "" {
						t.Fatalf("missing help response: %v", event)
					}
				case <-time.After(time.Second):
					t.Fatal("missing help response")
				}
			}
			if name != "bigbluebutton" {
				t.Logf("activation and /%s help passed", trigger)
			}
			if err := r.Disable(manifest.Id); err != nil {
				t.Fatal(err)
			}
		})
	}
}
