package mmplugins

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/avatar"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func TestScopedPluginUsersChannelsAndBotImages(t *testing.T) {
	opts := testOptions(t)
	ctx := context.Background()
	ten, err := opts.Store.CreateTenant(ctx, "Shared tenant")
	if err != nil {
		t.Fatal(err)
	}
	var users []*storage.User
	for _, name := range []string{"first.test", "sibling.test"} {
		dom, err := opts.Store.CreateDomain(ctx, ten.ID, name)
		if err != nil {
			t.Fatal(err)
		}
		u, err := opts.Store.CreateUser(ctx, &storage.User{TenantID: ten.ID, DomainID: dom.ID, Email: "owner@" + name, LocalPart: "owner", Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, u)
	}
	opts.TenantID, opts.DomainID = ten.ID, users[0].DomainID
	opts.Scope = "domain:" + ten.ID + ":" + opts.DomainID
	makeHost := func(id string) *hostAPI {
		return &hostAPI{unsupportedAPI: &unsupportedAPI{log: opts.Logger, pluginID: id}, manifest: &model.Manifest{Id: id}, runtime: &Runtime{options: opts}}
	}
	h := makeHost("github")
	if _, e := h.GetUser(chat.ID(users[0].ID)); e != nil {
		t.Fatal(e)
	}
	for _, u := range users[1:] {
		if _, e := h.GetUser(chat.ID(u.ID)); e == nil {
			t.Fatal("cross-domain user ID exposed")
		}
		if _, e := h.GetUserByEmail(u.Email); e == nil {
			t.Fatal("cross-domain email exposed")
		}
		if _, e := h.GetUserByUsername(modelUser(u).Username); e == nil {
			t.Fatal("cross-domain username exposed")
		}
	}
	first, err := opts.Chat.CreateChannel(ctx, users[0], "first-room", "First", "P")
	if err != nil {
		t.Fatal(err)
	}
	second, err := opts.Chat.CreateChannel(ctx, users[1], "sibling-room", "Sibling", "P")
	if err != nil {
		t.Fatal(err)
	}
	if _, e := h.GetChannel(second.ID); e == nil {
		t.Fatal("cross-domain channel exposed")
	}
	if _, e := h.AddChannelMember(first.ID, chat.ID(users[1].ID)); e == nil {
		t.Fatal("cross-domain member added")
	}
	if _, e := h.GetDirectChannel(chat.ID(users[0].ID), chat.ID(users[1].ID)); e == nil {
		t.Fatal("cross-domain direct channel created")
	}
	bot, e := h.CreateBot(&model.Bot{Username: "github", DisplayName: "GitHub"})
	if e != nil {
		t.Fatal(e)
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatal(err)
	}
	if e := h.SetProfileImage(bot.UserId, imageData.Bytes()); e != nil {
		t.Fatal(e)
	}
	got, e := h.GetProfileImage(bot.UserId)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(got, imageData.Bytes()) {
		t.Fatal("PNG profile image changed")
	}
	botUser, err := h.user(bot.UserId)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := avatar.Get(ctx, opts.Store, botUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Data != base64.StdEncoding.EncodeToString(got) || stored.Hash == "" {
		t.Fatal("native avatar storage missing")
	}
	if e := makeHost("gitlab").SetProfileImage(bot.UserId, imageData.Bytes()); e == nil {
		t.Fatal("other plugin's bot avatar overwritten")
	}
	if e := h.SetProfileImage(chat.ID(users[0].ID), imageData.Bytes()); e == nil {
		t.Fatal("human avatar overwritten")
	}
	if e := h.SetProfileImage(bot.UserId, []byte(`<svg onload="alert(1)"></svg>`)); e == nil {
		t.Fatal("active content accepted as avatar")
	}
	if e := h.SetProfileImage(bot.UserId, make([]byte, (1<<20)+1)); e == nil {
		t.Fatal("oversized avatar accepted")
	}
	otherOpts := opts
	otherOpts.DomainID = users[1].DomainID
	foreign := &hostAPI{unsupportedAPI: &unsupportedAPI{log: opts.Logger, pluginID: "github"}, manifest: &model.Manifest{Id: "github"}, runtime: &Runtime{options: otherOpts}}
	if _, e := foreign.GetProfileImage(bot.UserId); e == nil {
		t.Fatal("cross-domain bot avatar exposed through SDK")
	}
	events, stop := opts.Chat.Subscribe()
	defer stop()
	h.PublishWebSocketEvent("fixture", map[string]any{"value": "domain private"}, nil)
	ev := <-events
	if ev.TeamID != chat.ID(ten.ID) || ev.DomainID != users[0].DomainID {
		t.Fatal("plugin event lost its scope")
	}
	if h.GetConfig().LogSettings.EnableDiagnostics == nil || *h.GetConfig().LogSettings.EnableDiagnostics {
		t.Fatal("plugin telemetry enabled without configuration")
	}
}

func TestAPI12VersionGateAndLaterSDKMethods(t *testing.T) {
	opts := testOptions(t)
	r, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	archive := pluginArchive(t, map[string]any{"id": "future-plugin", "name": "Future", "version": "1.0.0", "min_server_version": "13.0.0", "server": map[string]any{"executable": "server/plugin"}}, map[string][]byte{"server/plugin": []byte("not executable")})
	if _, err := r.Install(bytes.NewReader(archive), false); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Enable("future-plugin"); err == nil || !strings.Contains(err.Error(), "plugin requires Mattermost 13.0.0; host API target is 12.0.0") {
		t.Fatal("future minimum server version ignored", err)
	}
	h := &hostAPI{unsupportedAPI: &unsupportedAPI{log: opts.Logger, pluginID: "fixture"}, runtime: r, manifest: &model.Manifest{Id: "fixture"}}
	if h.GetServerVersion() != "12.0.0" {
		t.Fatal("incorrect plugin compatibility target")
	}
	if _, e := h.GetThreadFollowers("fixture"); e == nil || e.StatusCode != 501 {
		t.Fatal("unimplemented SDK method did not return 501")
	}
}
