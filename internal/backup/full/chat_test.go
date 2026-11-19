package full

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mattermost"
	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/vks"
	"gopkg.in/yaml.v3"
)

func chatRequest(t *testing.T, h http.Handler, method, path, token, contentType string, body io.Reader, status int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "/api/v4"+path, body)
	r.Header.Set("Content-Type", contentType)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
	}
	return w
}

func chatJSON(t *testing.T, h http.Handler, method, path, token string, body any, status int) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	must(t, err)
	return chatRequest(t, h, method, path, token, "application/json", bytes.NewReader(data), status)
}

func responseID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var model struct{ ID string }
	must(t, json.Unmarshal(w.Body.Bytes(), &model))
	if model.ID == "" {
		t.Fatal("missing response identity")
	}
	return model.ID
}

func openChatAPI(t *testing.T, cfg *config.Config) (storage.Driver, *mattermost.API, *auth.Layer) {
	t.Helper()
	st, err := storage.Open(context.Background(), cfg.Storage)
	must(t, err)
	layer := auth.NewLayer(st, cfg.LDAP, cfg.MFA, cfg.OIDC)
	api := mattermost.New(st, layer, slog.New(slog.NewTextHandler(io.Discard, nil)), "https://chat.example.test")
	must(t, api.SetAttachmentDirectory(filepath.Join(filepath.Dir(cfg.Storage.SQLite.Path), ".mattermost-attachments")))
	return st, api, layer
}

func TestFullSnapshotRestoresMattermostChatAndSessions(t *testing.T) {
	f := newFullFixture(t)
	ctx := context.Background()
	f.cfg.Mattermost.Enabled = true
	f.cfg.Mattermost.PluginDirectory = filepath.Join(f.root, "plugins")
	f.cfg.VKS.SigningKeyFile = filepath.Join(f.root, "vks", "signing.pem")
	f.cfg.VKS.TURNSecretFile = filepath.Join(f.root, "vks", "turn.secret")
	f.put(t, "plugins/plugin.json", `{"id":"fixture"}`)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	must(t, err)
	f.put(t, "vks/signing.pem", string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})))
	f.put(t, "vks/turn.secret", strings.Repeat("fixture-turn-secret-", 3))
	f.cfg.VKS.Enabled = true
	f.cfg.VKS.PublicURL = "https://meet.example.test"
	f.cfg.VKS.Upstream = "http://127.0.0.1:8000"
	f.cfg.VKS.KeyID = "fixture-key"
	f.cfg.VKS.TURNURLs = []string{"turn:meet.example.test:3478?transport=udp"}
	data, err := yaml.Marshal(f.cfg)
	must(t, err)
	must(t, os.WriteFile(f.cfgFile, data, 0600))
	st, api, layer := openChatAPI(t, f.cfg)
	defer func() {
		if st != nil {
			st.Close()
		}
	}()
	login := func() string {
		w := chatJSON(t, api, "POST", "/users/login", "", map[string]string{"login_id": "owner@example.test", "password": "fixture-password"}, 200)
		if w.Header().Get("Token") == "" {
			t.Fatal("missing login token")
		}
		return w.Header().Get("Token")
	}
	current, other := login(), login()
	token, err := st.GetOAuthTokenByAccess(ctx, other)
	must(t, err)
	readerPair, err := layer.Tokens.IssueTokens(ctx, f.reader)
	must(t, err)
	reader := readerPair.AccessToken
	owner, err := st.GetUserByID(ctx, f.user)
	must(t, err)
	ten, err := st.CreateTenant(ctx, "foreign")
	must(t, err)
	dom, err := st.CreateDomain(ctx, ten.ID, "foreign.test")
	must(t, err)
	outsider, err := st.CreateUser(ctx, &storage.User{TenantID: ten.ID, DomainID: dom.ID, Email: "outsider@foreign.test", LocalPart: "outsider", Enabled: true})
	must(t, err)
	foreignPair, err := layer.Tokens.IssueTokens(ctx, outsider.ID)
	must(t, err)
	foreign := foreignPair.AccessToken
	dm := responseID(t, chatJSON(t, api, "POST", "/channels/direct", current, []string{chat.ID(f.user), chat.ID(f.reader)}, 201))
	public := responseID(t, chatJSON(t, api, "POST", "/channels", current, map[string]string{"team_id": chat.ID(owner.TenantID), "name": "recovery-room", "display_name": "Recovery", "type": "O"}, 201))
	chatJSON(t, api, "POST", "/channels/"+public+"/members", reader, map[string]string{"user_id": chat.ID(f.reader)}, 201)
	meeting, err := (vks.Meetings{Store: st}).CreateForChannel(ctx, owner, dm)
	must(t, err)
	beforeJoin, err := (vks.Meetings{Store: st}).JoinToken(ctx, key, f.cfg.VKS.KeyID, "meet.example.test", meeting, owner, time.Now())
	must(t, err)
	secret, err := os.ReadFile(f.cfg.VKS.TURNSecretFile)
	must(t, err)
	iceTime := time.Now().Truncate(time.Second)
	beforeICE, err := (vks.ICEConfig{TURNURLs: f.cfg.VKS.TURNURLs, Secret: string(secret)}).Credentials(iceTime, owner.TenantID, owner.DomainID, owner.ID)
	must(t, err)
	on := true
	gateway := mmsettings.TPNSURL
	policy := &mmsettings.Service{Store: st}
	must(t, policy.Save(ctx, mmsettings.Scope{}, mmsettings.Overrides{Push: &mmsettings.PushOverrides{Enabled: &on, ProxyURL: &gateway}}))
	chatJSON(t, api, "PUT", "/users/sessions/device", reader, map[string]string{"device_id": "apple_rn-v2:" + strings.Repeat("a", 64)}, 200)
	var photo bytes.Buffer
	must(t, png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 24, 48))))
	upload := chatRequest(t, api, "POST", "/files?channel_id="+dm+"&filename=portrait.png", current, "image/png", bytes.NewReader(photo.Bytes()), 201)
	var files struct {
		FileInfos []struct{ ID string } `json:"file_infos"`
	}
	must(t, json.Unmarshal(upload.Body.Bytes(), &files))
	if len(files.FileInfos) != 1 {
		t.Fatal("missing uploaded file")
	}
	file := files.FileInfos[0].ID
	input := map[string]any{"channel_id": dm, "message": "Recovered portrait", "file_ids": []string{file}, "pending_post_id": "recovery-idempotency-1"}
	post := responseID(t, chatJSON(t, api, "POST", "/posts", current, input, 201))
	chatJSON(t, api, "PUT", "/users/me/preferences", current, []map[string]string{{"category": "favorite_channel", "name": public, "value": "true"}}, 200)
	unread := chatJSON(t, api, "GET", "/users/me/channels/"+dm+"/unread", reader, nil, 200).Body.String()
	must(t, st.Close())
	st = nil
	started := time.Now()
	manifest, err := f.snapshot(ctx)
	must(t, err)
	if manifest.Counts["chat_posts"] != 1 || manifest.Counts["chat_files"] != 1 || manifest.Counts["chat_sessions"] < 2 || manifest.Counts["chat_push_devices"] != 1 || manifest.Counts["chat_push_jobs"] != 1 {
		t.Fatal("chat datasets missing from snapshot")
	}
	must(t, os.Rename(f.root, f.root+".offline"))
	target := filepath.Join(f.parent, "recovered-chat")
	_, err = Restore(ctx, RestoreOptions{Input: f.output, Identity: f.keyFile, Target: target})
	must(t, err)
	cfg, err := config.Load(filepath.Join(target, "config.yaml"))
	must(t, err)
	if cfg.VKS.SigningKeyFile != filepath.Join(target, "vks/signing.pem") || cfg.VKS.TURNSecretFile != filepath.Join(target, "vks/turn.secret") || cfg.Mattermost.PluginDirectory != filepath.Join(target, "plugins") {
		t.Fatal("chat/call paths still refer to source installation")
	}
	for _, name := range []string{cfg.VKS.SigningKeyFile, cfg.VKS.TURNSecretFile, filepath.Join(cfg.Mattermost.PluginDirectory, "plugin.json")} {
		if _, err := os.Stat(name); err != nil {
			t.Fatal(err)
		}
	}
	st, api, _ = openChatAPI(t, cfg)
	for _, table := range []string{"chat_push_jobs", "chat_push_devices"} {
		var count int
		must(t, st.DB().QueryRow("SELECT COUNT(*) FROM "+table).Scan(&count))
		if count != 1 {
			t.Fatal("push delivery state lost during recovery")
		}
	}
	recoveredKey, err := vks.LoadSigningKey(cfg.VKS.SigningKeyFile)
	must(t, err)
	if room, user, err := vks.VerifyJoinToken(&recoveredKey.PublicKey, cfg.VKS.KeyID, "meet.example.test", beforeJoin, time.Now()); err != nil || room != meeting.Room || user != owner.ID {
		t.Fatal("conference signing identity changed during recovery", err)
	}
	meetings := vks.Meetings{Store: st}
	recoveredMeeting, err := meetings.Get(ctx, meeting.Room)
	must(t, err)
	member, err := st.GetUserByID(ctx, f.reader)
	must(t, err)
	if !meetings.CanJoin(ctx, recoveredMeeting, member) || meetings.CanJoin(ctx, recoveredMeeting, outsider) {
		t.Fatal("conference membership changed during recovery")
	}
	_, err = meetings.JoinToken(ctx, recoveredKey, cfg.VKS.KeyID, "meet.example.test", recoveredMeeting, member, time.Now())
	must(t, err)
	secret, err = os.ReadFile(cfg.VKS.TURNSecretFile)
	must(t, err)
	afterICE, err := (vks.ICEConfig{TURNURLs: cfg.VKS.TURNURLs, Secret: string(secret)}).Credentials(iceTime, owner.TenantID, owner.DomainID, owner.ID)
	must(t, err)
	if !reflect.DeepEqual(beforeICE, afterICE) {
		t.Fatal("TURN credentials changed during recovery")
	}
	chatJSON(t, api, "GET", "/users/me", current, nil, 200)
	chatJSON(t, api, "GET", "/users/me", other, nil, 200)
	if got := chatJSON(t, api, "GET", "/users/me/channels/"+dm+"/unread", reader, nil, 200).Body.String(); got != unread {
		t.Fatal("unread state changed during recovery")
	}
	if got := responseID(t, chatJSON(t, api, "POST", "/posts", current, input, 201)); got != post {
		t.Fatal("retry duplicated recovered post")
	}
	got := chatJSON(t, api, "GET", "/posts/"+post, reader, nil, 200)
	if !strings.Contains(got.Body.String(), `"height":48`) || !strings.Contains(got.Body.String(), `"width":24`) {
		t.Fatal("portrait display dimensions lost")
	}
	for _, route := range []string{"/files/" + file, "/files/" + file + "/preview", "/files/" + file + "/thumbnail"} {
		w := chatRequest(t, api, "GET", route, reader, "", nil, 200)
		if !bytes.Equal(w.Body.Bytes(), photo.Bytes()) {
			t.Fatal("restored photo bytes changed")
		}
		chatRequest(t, api, "GET", route, foreign, "", nil, 404)
	}
	chatJSON(t, api, "GET", "/posts/"+post, foreign, nil, 404)
	chatJSON(t, api, "GET", "/channels/"+public, reader, nil, 200)
	if !strings.Contains(chatJSON(t, api, "GET", "/users/me/preferences", current, nil, 200).Body.String(), public) {
		t.Fatal("preferences lost")
	}
	chatJSON(t, api, "POST", "/users/sessions/revoke", current, map[string]string{"session_id": chat.ID(token.ID)}, 200)
	chatJSON(t, api, "GET", "/users/me", other, nil, 401)
	chatJSON(t, api, "GET", "/users/me", current, nil, 200)
	chatJSON(t, api, "POST", "/channels/members/me/view", reader, map[string]string{"channel_id": dm}, 200)
	var readState struct {
		Count int `json:"msg_count"`
	}
	must(t, json.Unmarshal(chatJSON(t, api, "GET", "/users/me/channels/"+dm+"/unread", reader, nil, 200).Body.Bytes(), &readState))
	if readState.Count != 0 {
		t.Fatal("cannot mark restored history read")
	}
	fresh := login()
	chatJSON(t, api, "GET", "/users/me", fresh, nil, 200)
	t.Logf("synthetic encrypted snapshot, restore and chat API acceptance completed in %s", time.Since(started).Round(time.Millisecond))
}

func newChatAttachmentFixture(t *testing.T) (*fullFixture, string) {
	t.Helper()
	f := newFullFixture(t)
	st, api, layer := openChatAPI(t, f.cfg)
	defer st.Close()
	pair, err := layer.Tokens.IssueTokens(context.Background(), f.user)
	must(t, err)
	channel := responseID(t, chatJSON(t, api, "POST", "/channels/direct", pair.AccessToken, []string{chat.ID(f.user), chat.ID(f.reader)}, 201))
	w := chatRequest(t, api, "POST", "/files?channel_id="+channel+"&filename=sample.txt", pair.AccessToken, "text/plain", strings.NewReader("fixture attachment"), 201)
	var files struct {
		FileInfos []struct{ ID string } `json:"file_infos"`
	}
	must(t, json.Unmarshal(w.Body.Bytes(), &files))
	if len(files.FileInfos) != 1 {
		t.Fatal("missing attachment")
	}
	return f, files.FileInfos[0].ID
}

func TestFullSnapshotRejectsIncompleteChatAttachments(t *testing.T) {
	for _, scenario := range []string{"missing", "truncated", "symlink", "invalid-id", "truncated-blob"} {
		t.Run(scenario, func(t *testing.T) {
			f, id := newChatAttachmentFixture(t)
			name := filepath.Join(f.root, ".mattermost-attachments", id)
			switch scenario {
			case "missing":
				must(t, os.Remove(name))
			case "truncated":
				must(t, os.WriteFile(name, []byte("short"), 0600))
			case "symlink":
				must(t, os.Rename(name, name+".original"))
				must(t, os.Symlink(name+".original", name))
			default:
				db, err := openDB(f.cfg.Storage.SQLite.Path, false)
				must(t, err)
				if scenario == "invalid-id" {
					_, err = db.Exec(`UPDATE chat_files SET id='../outside' WHERE id=?`, id)
				} else {
					_, err = db.Exec(`UPDATE chat_files SET on_disk=0,content=? WHERE id=?`, []byte("short"), id)
				}
				must(t, err)
				must(t, db.Close())
			}
			if _, err := f.snapshot(context.Background()); err == nil || !strings.Contains(err.Error(), "chat attachment") {
				t.Fatalf("incomplete chat snapshot accepted, or unrelated failure: %v", err)
			}
			if _, err := os.Stat(f.output); !os.IsNotExist(err) {
				t.Fatal("invalid snapshot was published")
			}
		})
	}
}

func TestFullSnapshotAcceptsLegacyChatAttachmentStorage(t *testing.T) {
	for _, scenario := range []string{"blob", "before-disk-migration", "before-chat-migration"} {
		t.Run(scenario, func(t *testing.T) {
			f, id := newChatAttachmentFixture(t)
			db, err := openDB(f.cfg.Storage.SQLite.Path, false)
			must(t, err)
			_, err = db.Exec(`UPDATE chat_files SET on_disk=0,content=? WHERE id=?`, []byte("fixture attachment"), id)
			must(t, err)
			must(t, os.Remove(filepath.Join(f.root, ".mattermost-attachments", id)))
			switch scenario {
			case "before-disk-migration":
				_, err = db.Exec(`ALTER TABLE chat_files DROP COLUMN on_disk`)
			case "before-chat-migration":
				_, err = db.Exec(`DROP TABLE chat_files`)
			}
			must(t, err)
			must(t, db.Close())
			_, err = f.snapshot(context.Background())
			must(t, err)
			_, err = Restore(context.Background(), RestoreOptions{Input: f.output, Identity: f.keyFile, Target: filepath.Join(f.parent, "legacy-verified"), VerifyOnly: true})
			must(t, err)
		})
	}
}

func TestFullSnapshotRejectsExternalChatAndCallPaths(t *testing.T) {
	for _, field := range []string{"signing-key", "turn-secret", "plugins"} {
		t.Run(field, func(t *testing.T) {
			f := newFullFixture(t)
			external := filepath.Join(f.parent, "external")
			if field == "plugins" {
				must(t, os.Mkdir(external, 0700))
				f.cfg.Mattermost.PluginDirectory = external
			} else {
				must(t, os.WriteFile(external, []byte("private fixture key"), 0600))
				if field == "signing-key" {
					f.cfg.VKS.SigningKeyFile = external
				} else {
					f.cfg.VKS.TURNSecretFile = external
				}
			}
			data, err := yaml.Marshal(f.cfg)
			must(t, err)
			must(t, os.WriteFile(f.cfgFile, data, 0600))
			if _, err = f.snapshot(context.Background()); err == nil || !strings.Contains(err.Error(), "outside installation root") {
				t.Fatalf("external dependency not rejected: %v", err)
			}
		})
	}
}
