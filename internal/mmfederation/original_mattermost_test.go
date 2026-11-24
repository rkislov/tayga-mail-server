// SPDX-License-Identifier: Apache-2.0
package mmfederation_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/mmfederation"
)

// Opt-in test for a disposable, unlicensed original Mattermost server. Never
// point this at a working workspace: it creates a user, team and test channel.
func TestOriginalMattermostREST(t *testing.T) {
	raw := os.Getenv("TMS_TEST_MATTERMOST_URL")
	if raw == "" {
		t.Skip("set TMS_TEST_MATTERMOST_URL to a disposable original Mattermost server")
	}
	target, e := url.Parse(raw)
	if e != nil || target.Scheme != "http" || target.Hostname() != "127.0.0.1" {
		t.Fatal("original Mattermost test accepts only an isolated loopback HTTP server")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	ready := false
	for i := 0; i < 120; i++ {
		response, e := client.Get(raw + "/api/v4/system/ping")
		if e == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(time.Second)
	}
	if !ready {
		t.Fatal("disposable Mattermost did not become ready")
	}
	call := func(method, path, token string, body any, status int, result any) http.Header {
		t.Helper()
		var reader io.Reader
		if body != nil {
			data, e := json.Marshal(body)
			if e != nil {
				t.Fatal(e)
			}
			reader = bytes.NewReader(data)
		}
		request, e := http.NewRequest(method, raw+"/api/v4"+path, reader)
		if e != nil {
			t.Fatal(e)
		}
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response, e := client.Do(request)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		if response.StatusCode != status {
			t.Fatalf("original Mattermost %s %s: HTTP %d expected %d", method, path, response.StatusCode, status)
		}
		if result != nil {
			if e = json.NewDecoder(response.Body).Decode(result); e != nil {
				t.Fatal(e)
			}
		}
		return response.Header
	}
	suffix := model.NewId()[:10]
	username := "fedtest" + suffix
	password := "Test-only-Original-9!" + suffix
	var user model.User
	call("POST", "/users", "", map[string]string{"username": username, "email": username + "@example.test", "password": password}, 201, &user)
	headers := call("POST", "/users/login", "", map[string]string{"login_id": username, "password": password}, 200, &user)
	token := headers.Get("Token")
	if token == "" {
		t.Fatal("original Mattermost did not issue an authentication token")
	}
	var team model.Team
	call("POST", "/teams", token, map[string]string{"name": "fed" + suffix, "display_name": "Federation acceptance", "type": "O"}, 201, &team)
	var channel model.Channel
	call("POST", "/channels", token, map[string]string{"team_id": team.Id, "name": "shared" + suffix, "display_name": "Shared acceptance", "type": "O"}, 201, &channel)
	var bot model.Bot
	call("POST", "/bots", token, map[string]string{"username": "bridge" + suffix, "display_name": "Acceptance bridge bot"}, 201, &bot)
	call("POST", "/teams/"+team.Id+"/members", token, map[string]string{"team_id": team.Id, "user_id": bot.UserId}, 201, nil)
	call("POST", "/channels/"+channel.Id+"/members", token, map[string]string{"user_id": bot.UserId}, 201, nil)
	var access model.UserAccessToken
	call("POST", "/users/"+bot.UserId+"/tokens", token, map[string]string{"description": "isolated federation acceptance"}, 200, &access)
	// Production requires HTTPS. Terminate TLS in the isolated test proxy while
	// keeping the disposable original server on its loopback-only Docker port.
	proxy := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(target))
	defer proxy.Close()
	local := setup(t, "original")
	local.service.HTTP = proxy.Client()
	peer, e := local.service.CreatePeer(local.ctx, mmfederation.Peer{TenantID: local.user.TenantID, DomainID: local.user.DomainID, OwnerID: local.user.ID, Name: "original", Mode: "rest", SiteURL: proxy.URL}, mmfederation.Credentials{AccessToken: access.Token})
	if e != nil {
		t.Fatal(e)
	}
	link, e := local.service.CreateLink(local.ctx, peer, mmfederation.Link{ChannelID: local.channel.ID, RemoteChannelID: channel.Id}, true)
	if e != nil {
		t.Fatal(e)
	}
	staged, e := local.service.Chat.StageAttachment("acceptance.txt", strings.NewReader("original Mattermost file bytes"), 100)
	if e != nil {
		t.Fatal(e)
	}
	defer staged.Discard()
	files, e := local.service.Chat.UploadAttachments(local.ctx, local.user, local.channel.ID, []chat.StagedUpload{staged})
	if e != nil {
		t.Fatal(e)
	}
	sent, e := local.service.Chat.SendWithFiles(local.ctx, local.user, local.channel.ID, "", "to original", "original-test", nil, []string{files[0].ID})
	if e != nil {
		t.Fatal(e)
	}
	if e = local.service.Sync(local.ctx, peer, link); e != nil {
		t.Fatal(e)
	}
	var foreign string
	if e = local.store.DB().QueryRow(`SELECT remote_post_id FROM chat_federation_posts WHERE link_id=? AND local_post_id=?`, link.ID, sent.ID).Scan(&foreign); e != nil {
		t.Fatal(e)
	}
	var received model.Post
	call("GET", "/posts/"+foreign, token, nil, 200, &received)
	if !strings.Contains(received.Message, "to original") || len(received.FileIds) != 1 {
		t.Fatal("original Mattermost did not receive the message and file")
	}
	request, _ := http.NewRequest("GET", raw+"/api/v4/files/"+received.FileIds[0], nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, e := client.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	data, e := io.ReadAll(response.Body)
	response.Body.Close()
	if e != nil || response.StatusCode != 200 || string(data) != "original Mattermost file bytes" {
		t.Fatal("original received corrupt file", e)
	}
	var upload bytes.Buffer
	writer := multipart.NewWriter(&upload)
	_ = writer.WriteField("channel_id", channel.Id)
	part, e := writer.CreateFormFile("files", "from-original.txt")
	if e != nil {
		t.Fatal(e)
	}
	_, _ = part.Write([]byte("reverse file bytes"))
	_ = writer.Close()
	request, _ = http.NewRequest("POST", raw+"/api/v4/files", &upload)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, e = client.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	var uploaded struct {
		FileInfos []model.FileInfo `json:"file_infos"`
	}
	e = json.NewDecoder(response.Body).Decode(&uploaded)
	response.Body.Close()
	if e != nil || response.StatusCode != 201 || len(uploaded.FileInfos) != 1 {
		t.Fatal("original test upload failed", e)
	}
	var reply model.Post
	call("POST", "/posts", token, map[string]any{"channel_id": channel.Id, "root_id": foreign, "message": "from original", "file_ids": []string{uploaded.FileInfos[0].Id}}, 201, &reply)
	if e = local.service.Sync(local.ctx, peer, link); e != nil {
		t.Fatal(e)
	}
	posts, e := local.service.Chat.Posts(local.ctx, local.user, local.channel.ID, 100, 0, 0)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, post := range posts {
		if strings.Contains(post.Message, "from original") {
			found = post.RootID == sent.ID
			if len(post.Attachments) != 1 {
				t.Fatal("reverse attachment missing")
			}
			_, body, e := local.service.Chat.Attachment(local.ctx, local.user, post.Attachments[0].ID, true)
			if e != nil {
				t.Fatal(e)
			}
			data, e := io.ReadAll(body)
			body.Close()
			if e != nil || string(data) != "reverse file bytes" {
				t.Fatal("reverse file corrupt", e)
			}
		}
	}
	if !found {
		t.Fatal("original reply lost its thread")
	}
	if e = local.service.Sync(local.ctx, peer, link); e != nil {
		t.Fatal(e)
	}
	var history struct {
		Posts map[string]*model.Post `json:"posts"`
	}
	call("GET", "/channels/"+channel.Id+"/posts", token, nil, 200, &history)
	matches := 0
	for _, post := range history.Posts {
		if strings.Contains(post.Message, "to original") {
			matches++
		}
	}
	if matches != 1 {
		t.Fatal("duplicate message on original Mattermost", matches)
	}
}
