// SPDX-License-Identifier: Apache-2.0
package mmfederation_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mattermost"
	"github.com/tayga/tms/internal/mmfederation"
	"github.com/tayga/tms/internal/storage"
)

type fixture struct {
	ctx     context.Context
	store   storage.Driver
	service *mmfederation.Service
	api     *mattermost.API
	user    *storage.User
	channel chat.Channel
	server  *httptest.Server
	cipher  cipher.AEAD
	token   string
}

func setup(t *testing.T, name string) *fixture {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	st, e := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(root, "test.db")}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	tenant, e := st.CreateTenant(ctx, name)
	if e != nil {
		t.Fatal(e)
	}
	domain, e := st.CreateDomain(ctx, tenant.ID, name+".example")
	if e != nil {
		t.Fatal(e)
	}
	user, e := st.CreateUser(storage.WithoutWelcomeMail(ctx), &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "owner@" + domain.Name, LocalPart: "owner", DisplayName: name, Enabled: true, AuthSource: "local"})
	if e != nil {
		t.Fatal(e)
	}
	cfg := config.Default()
	layer := auth.NewLayer(st, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	pair, e := layer.Tokens.IssueTokens(ctx, user.ID)
	if e != nil {
		t.Fatal(e)
	}
	api := mattermost.New(st, layer, slog.New(slog.NewTextHandler(io.Discard, nil)), "https://placeholder.example")
	if e = api.SetAttachmentDirectory(filepath.Join(root, "files")); e != nil {
		t.Fatal(e)
	}
	key := sha256.Sum256([]byte(name))
	block, e := aes.NewCipher(key[:])
	if e != nil {
		t.Fatal(e)
	}
	aead, e := cipher.NewGCM(block)
	if e != nil {
		t.Fatal(e)
	}
	if e = api.EnableFederation(aead, []string{"127.0.0.0/8"}); e != nil {
		t.Fatal(e)
	}
	service := api.Federation()
	server := httptest.NewTLSServer(api)
	t.Cleanup(server.Close)
	service.PublicURL = server.URL
	service.HTTP = server.Client()
	channel, e := service.Chat.CreateChannel(ctx, user, "shared", "Shared", "O")
	if e != nil {
		t.Fatal(e)
	}
	return &fixture{ctx: ctx, store: st, service: service, api: api, user: user, channel: channel, server: server, cipher: aead, token: pair.AccessToken}
}
func nativePair(t *testing.T) (*fixture, *fixture, mmfederation.Peer, mmfederation.Peer, mmfederation.Link, mmfederation.Link) {
	t.Helper()
	a, b := setup(t, "alpha"), setup(t, "beta")
	pa, e := a.service.CreatePeer(a.ctx, mmfederation.Peer{TenantID: a.user.TenantID, DomainID: a.user.DomainID, OwnerID: a.user.ID, Name: "beta", Mode: "native", SiteURL: b.server.URL}, mmfederation.Credentials{})
	if e != nil {
		t.Fatal(e)
	}
	code, e := a.service.Invite(a.ctx, pa, "test-passphrase-12345")
	if e != nil {
		t.Fatal(e)
	}
	pb, e := b.service.AcceptInvite(b.ctx, mmfederation.Peer{TenantID: b.user.TenantID, DomainID: b.user.DomainID, OwnerID: b.user.ID, Name: "alpha"}, code, "test-passphrase-12345")
	if e != nil {
		t.Fatal(e)
	}
	pa, e = a.service.Peer(a.ctx, pa.ID)
	if e != nil || !pa.Confirmed || !pb.Confirmed {
		t.Fatal("native confirmation failed", pa, pb, e)
	}
	la, e := a.service.CreateLink(a.ctx, pa, mmfederation.Link{ChannelID: a.channel.ID, RemoteChannelID: a.channel.ID}, true)
	if e != nil {
		t.Fatal(e)
	}
	lb, e := b.service.CreateLink(b.ctx, pb, mmfederation.Link{ChannelID: b.channel.ID, RemoteChannelID: a.channel.ID}, true)
	if e != nil {
		t.Fatal(e)
	}
	return a, b, pa, pb, la, lb
}
func TestNativePairThreadsFilesAndRestart(t *testing.T) {
	a, b, pa, pb, la, lb := nativePair(t)
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=")
	staged, e := a.service.Chat.StageAttachment("photo.png", bytes.NewReader(png), int64(len(png)))
	if e != nil {
		t.Fatal(e)
	}
	defer staged.Discard()
	files, e := a.service.Chat.UploadAttachments(a.ctx, a.user, a.channel.ID, []chat.StagedUpload{staged})
	if e != nil {
		t.Fatal(e)
	}
	sent, e := a.service.Chat.SendWithFiles(a.ctx, a.user, a.channel.ID, "", "Hello beta", "pending1", nil, []string{files[0].ID})
	if e != nil {
		t.Fatal(e)
	}
	if e = a.service.Sync(a.ctx, pa, la); e != nil {
		t.Fatal(e)
	}
	posts, e := b.service.Chat.Posts(b.ctx, b.user, b.channel.ID, 100, 0, 0)
	if e != nil || len(posts) != 1 || posts[0].Message != "Hello beta" || len(posts[0].Attachments) != 1 {
		t.Fatal(posts, e)
	}
	remote, e := b.store.GetUserByID(b.ctx, posts[0].UserID)
	if e != nil || remote.Enabled || remote.AuthSource != "federated" {
		t.Fatal("remote identity became a login account", remote, e)
	}
	_, data, e := b.service.Chat.Attachment(b.ctx, b.user, posts[0].Attachments[0].ID, true)
	if e != nil {
		t.Fatal(e)
	}
	got, e := io.ReadAll(data)
	data.Close()
	if e != nil || !bytes.Equal(got, png) {
		t.Fatal("image bytes changed", e)
	}
	reply, e := b.service.Chat.Send(b.ctx, b.user, b.channel.ID, posts[0].ID, "Hello alpha", "reply1")
	if e != nil {
		t.Fatal(e)
	}
	if e = b.service.Sync(b.ctx, pb, lb); e != nil {
		t.Fatal(e)
	}
	original, e := a.service.Chat.Posts(a.ctx, a.user, a.channel.ID, 100, 0, 0)
	if e != nil || len(original) != 2 {
		t.Fatal(original, e)
	}
	found := false
	for _, post := range original {
		if post.Message == reply.Message {
			found = post.RootID == sent.ID
		}
	}
	if !found {
		t.Fatal("thread root not preserved", original)
	}
	// Restart the protocol engine; durable mappings and cursors must prevent echoes.
	restarted := mmfederation.New(a.store, a.service.Chat, a.cipher, a.server.URL, a.server.Client(), nil)
	if e = restarted.Sync(a.ctx, pa, la); e != nil {
		t.Fatal(e)
	}
	posts, e = b.service.Chat.Posts(b.ctx, b.user, b.channel.ID, 100, 0, 0)
	if e != nil || len(posts) != 2 {
		t.Fatal("duplicate after restart", posts, e)
	}
	if e = b.service.DisableLink(b.ctx, pb, lb.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = a.service.Chat.Send(a.ctx, a.user, a.channel.ID, "", "must not arrive", "after-disable"); e != nil {
		t.Fatal(e)
	}
	if e = a.service.Sync(a.ctx, pa, la); e == nil {
		t.Fatal("remote link revocation ignored")
	}
	posts, e = b.service.Chat.Posts(b.ctx, b.user, b.channel.ID, 100, 0, 0)
	if e != nil || len(posts) != 2 {
		t.Fatal("disabled link received data", posts, e)
	}
}
func TestRESTPairFilesAndNoEcho(t *testing.T) {
	a, b := setup(t, "restalpha"), setup(t, "restbeta")
	p, e := a.service.CreatePeer(a.ctx, mmfederation.Peer{TenantID: a.user.TenantID, DomainID: a.user.DomainID, OwnerID: a.user.ID, Name: "rest-beta", Mode: "rest", SiteURL: b.server.URL}, mmfederation.Credentials{AccessToken: b.token})
	if e != nil {
		t.Fatal(e)
	}
	l, e := a.service.CreateLink(a.ctx, p, mmfederation.Link{ChannelID: a.channel.ID, RemoteChannelID: b.channel.ID}, true)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.service.Chat.Send(a.ctx, a.user, a.channel.ID, "", "outbound", "rest-out"); e != nil {
		t.Fatal(e)
	}
	if e = a.service.Sync(a.ctx, p, l); e != nil {
		t.Fatal(e)
	}
	posts, e := b.service.Chat.Posts(b.ctx, b.user, b.channel.ID, 100, 0, 0)
	if e != nil || len(posts) != 1 || !strings.Contains(posts[0].Message, "outbound") {
		t.Fatal(posts, e)
	}
	staged, e := b.service.Chat.StageAttachment("notes.txt", strings.NewReader("safe payload"), 100)
	if e != nil {
		t.Fatal(e)
	}
	defer staged.Discard()
	files, e := b.service.Chat.UploadAttachments(b.ctx, b.user, b.channel.ID, []chat.StagedUpload{staged})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = b.service.Chat.SendWithFiles(b.ctx, b.user, b.channel.ID, posts[0].ID, "inbound", "rest-in", nil, []string{files[0].ID}); e != nil {
		t.Fatal(e)
	}
	if e = a.service.Sync(a.ctx, p, l); e != nil {
		t.Fatal(e)
	}
	imported, e := a.service.Chat.Posts(a.ctx, a.user, a.channel.ID, 100, 0, 0)
	if e != nil || len(imported) != 2 {
		t.Fatal(imported, e)
	}
	for _, post := range imported {
		if strings.Contains(post.Message, "inbound") {
			if len(post.Attachments) != 1 || post.RootID == "" {
				t.Fatal("REST files or root missing", post)
			}
		}
	}
	if e = a.service.Sync(a.ctx, p, l); e != nil {
		t.Fatal(e)
	}
	again, e := b.service.Chat.Posts(b.ctx, b.user, b.channel.ID, 100, 0, 0)
	if e != nil || len(again) != 2 {
		t.Fatal("REST echo", again, e)
	}
	var stored string
	if e = a.store.DB().QueryRow(`SELECT secret FROM chat_federation_peers WHERE id=?`, p.ID).Scan(&stored); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(stored, b.token) {
		t.Fatal("clear token persisted")
	}
	if e = a.service.Disable(a.ctx, p); e != nil {
		t.Fatal(e)
	}
	if e = a.service.Sync(a.ctx, p, l); e == nil {
		t.Fatal("disabled peer still synchronized")
	}
}
func TestFederationScopeAndTransportGuards(t *testing.T) {
	f := setup(t, "guards")
	otherDomain, e := f.store.CreateDomain(f.ctx, f.user.TenantID, "sibling.example")
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.service.CreatePeer(f.ctx, mmfederation.Peer{TenantID: f.user.TenantID, DomainID: otherDomain.ID, OwnerID: f.user.ID, Name: "escape", Mode: "rest", SiteURL: "https://public.example"}, mmfederation.Credentials{AccessToken: "secret"})
	if e == nil {
		t.Fatal("owner crossed sibling domain")
	}
	for _, u := range []string{"http://host.example", "https://user:secret@host.example", "https://host.example/#fragment", "https://host.example/?token=x"} {
		if _, e = f.service.CreatePeer(f.ctx, mmfederation.Peer{TenantID: f.user.TenantID, DomainID: f.user.DomainID, OwnerID: f.user.ID, Name: model.NewId(), Mode: "rest", SiteURL: u}, mmfederation.Credentials{AccessToken: "token"}); e == nil {
			t.Fatal("unsafe URL accepted", u)
		}
	}
	client, e := mmfederation.NewHTTPClient(nil)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", f.server.URL, nil)
	if response, e := client.Do(req); e == nil {
		response.Body.Close()
		t.Fatal("private peer bypassed explicit CIDR allowlist")
	}
	req, _ = http.NewRequestWithContext(f.ctx, "POST", f.server.URL+"/api/v4/remotecluster/msg", strings.NewReader(`{}`))
	req.Header.Set("X-RemoteCluster-Id", model.NewId())
	req.Header.Set("X-RemoteCluster-Token", "forged")
	response, e := f.server.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal(response.Status)
	}
}
