package httpapi_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/vks"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVKSAuthenticatedRoomBrandingAndICE(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	dir := t.TempDir()
	cfg.Storage.SQLite.Path = filepath.Join(dir, "db")
	cfg.FlowSync.Enabled = false
	cfg.Mattermost.Enabled = true
	cfg.HTTP.PublicURL = "https://mail.example.test"
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ready":true}`))
	}))
	defer upstream.Close()
	cfg.VKS = config.VKSConfig{Upstream: upstream.URL, Enabled: true, PublicURL: "https://meet.example.test", SigningKeyFile: filepath.Join(dir, "key"), KeyID: "test-key", STUNURLs: []string{"stun:meet.example.test:3478"}, TURNURLs: []string{"turn:meet.example.test:3478?transport=udp"}, TURNSecretFile: filepath.Join(dir, "secret"), TURNTTL: time.Hour}
	os.WriteFile(cfg.VKS.SigningKeyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600)
	os.WriteFile(cfg.VKS.TURNSecretFile, bytes.Repeat([]byte("s"), 40), 0600)
	st, e := storage.Open(ctx, cfg.Storage)
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	layer := auth.NewLayer(st, cfg.LDAP, cfg.MFA, cfg.OIDC)
	tenant, _ := st.CreateTenant(ctx, "A")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "example.test")
	sibling, _ := st.CreateDomain(ctx, tenant.ID, "sibling.test")
	makeUser := func(name string, d *storage.Domain) (*storage.User, string) {
		u, e := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: d.ID, Email: name + "@" + d.Name, LocalPart: name, Enabled: true, Roles: storage.RoleUser, AuthSource: "local"})
		if e != nil {
			t.Fatal(e)
		}
		p, e := layer.Tokens.IssueTokens(ctx, u.ID)
		if e != nil {
			t.Fatal(e)
		}
		return u, p.AccessToken
	}
	owner, token := makeUser("owner", domain)
	participant, member := makeUser("member", domain)
	_, outsider := makeUser("outsider", domain)
	_, foreign := makeUser("foreign", sibling)
	h := httpapi.New(cfg, slog.Default(), st, layer, nil, nil, nil).Handler()
	send := func(method, path, authToken, body string, want int) []byte {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if strings.Contains(path, "conference-request") {
			r.Host = "meet.example.test"
		}
		if authToken != "" {
			r.Header.Set("Authorization", "Bearer "+authToken)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d want %d %s", method, path, w.Code, want, w.Body)
		}
		return w.Body.Bytes()
	}
	request := func(method, path, authToken, body string, want int) map[string]any {
		t.Helper()
		var out map[string]any
		json.Unmarshal(send(method, path, authToken, body, want), &out)
		return out
	}
	request("GET", "/api/v1/vks/ice", "", "", 401)
	ice := request("GET", "/api/v1/vks/ice", token, "", 200)
	if len(ice["ice_servers"].([]any)) != 2 {
		t.Fatal(ice)
	}
	bad, _ := json.Marshal(map[string]any{"title": "Denied", "participants": []string{"foreign@" + sibling.Name}})
	request("POST", "/api/v1/vks/meetings", token, string(bad), 400)
	body, _ := json.Marshal(map[string]any{"title": "Test", "participants": []string{participant.Email}})
	meeting := request("POST", "/api/v1/vks/meetings", token, string(body), 201)
	room := meeting["room"].(string)
	request("POST", "/api/v1/vks/join/"+room, outsider, "", 404)
	request("POST", "/api/v1/vks/join/"+room, foreign, "", 404)
	st.PutSetting(ctx, "branding.tenants."+tenant.ID, `{"enabled":true,"name":"Tenant"}`)
	st.PutSetting(ctx, "branding.domains."+domain.ID, `{"enabled":true,"name":"Domain"}`)
	joined := request("POST", "/api/v1/vks/join/"+room, member, "", 200)
	focusBody := `{"room":"` + room + `@conference.meet.example.test"}`
	request("POST", "/conference-request/v1", "", focusBody, 401)
	request("POST", "/conference-request/v1", "invalid", focusBody, 401)
	request("POST", "/conference-request/v1", joined["jwt"].(string), focusBody, 200)
	request("POST", "/conference-request/v1", joined["jwt"].(string), `{"room":"other@conference.meet.example.test"}`, 403)
	request("POST", "//conference-request/v1", "", focusBody, 404)
	if joined["branding"].(map[string]any)["name"] != "Domain" {
		t.Fatal(joined)
	}
	request("GET", "/api/v1/vks/branding?conferenceFqn="+room, "", "", 200)
	team := chat.ID(tenant.ID)
	channel := request("POST", "/api/v4/channels", token,
		`{"team_id":"`+team+`","name":"meeting-check","display_name":"Совещание","type":"O"}`, 201)
	id := channel["id"].(string)
	command := `{"team_id":"` + team + `","channel_id":"` + id + `","command":"/meet"}`
	request("POST", "/api/v4/commands/execute", outsider, command, 404)
	const apps = "/plugins/com.mattermost.apps/api/v1"
	bindings := apps + "/bindings?channel_id=" + id + "&team_id=" + team + "&user_agent=mobile"
	cfgClient := request("GET", "/api/v4/config/client?format=old", token, "", 200)
	if cfgClient["FeatureFlagAppsEnabled"] != "true" {
		t.Fatal("Apps feature flag missing")
	}
	send("GET", bindings, "", "", 401)
	if got := strings.TrimSpace(string(send("GET", bindings+"&user_id="+chat.ID(participant.ID), member, "", 200))); got != "[]" {
		t.Fatalf("unjoined channel advertised Jitsi action: %s", got)
	}
	var menu []struct {
		Location string
		Bindings []struct {
			AppID  string `json:"app_id"`
			Label  string
			Submit struct{ Path string }
		}
	}
	if err := json.Unmarshal(send("GET", bindings+"&user_id="+chat.ID(owner.ID), token, "", 200), &menu); err != nil ||
		len(menu) != 1 || menu[0].Location != "/channel_header" || len(menu[0].Bindings) != 1 ||
		menu[0].Bindings[0].Label != "Начать звонок Jitsi" {
		t.Fatalf("invalid channel action: %+v (%v)", menu, err)
	}
	action := menu[0].Bindings[0]
	callBody := `{"path":"` + action.Submit.Path + `","context":{"app_id":"` + action.AppID + `","channel_id":"` + id + `","team_id":"` + team + `","user_agent":"mobile","track_as_submit":true}}`
	request("POST", apps+"/call", outsider, callBody, 404)
	result := request("POST", apps+"/call", token, callBody, 200)
	if result["type"] != "navigate" {
		t.Fatal(result)
	}
	link, e := url.Parse(result["navigate_to_url"].(string))
	if e != nil || link.Host != "mail.example.test" || link.Query().Get("app") != "vks" || link.Query().Has("jwt") {
		t.Fatal("invalid call link")
	}
	callRoom := link.Query().Get("room")
	request("POST", "/api/v1/vks/join/"+callRoom, member, "", 404)
	request("POST", "/api/v4/channels/"+id+"/members", member, `{"user_id":"`+chat.ID(participant.ID)+`"}`, 201)
	callJoin := request("POST", "/api/v1/vks/join/"+callRoom, member, "", 200)
	history := request("GET", "/api/v4/channels/"+id+"/posts", member, "", 200)
	if len(history["order"].([]any)) != 1 || !strings.Contains(history["posts"].(map[string]any)[history["order"].([]any)[0].(string)].(map[string]any)["message"].(string), link.String()) {
		t.Fatal("call invitation missing")
	}
	focusCall := `{"room":"` + callRoom + `@conference.meet.example.test"}`
	request("POST", "/conference-request/v1", callJoin["jwt"].(string), focusCall, 200)
	if _, err := st.DB().Exec("DELETE FROM chat_members WHERE channel_id=? AND user_id=?", id, participant.ID); err != nil {
		t.Fatal(err)
	}
	request("POST", "/api/v1/vks/join/"+callRoom, member, "", 404)
	request("POST", "/conference-request/v1", callJoin["jwt"].(string), focusCall, 403)

	policy := vks.Policies{Store: st}
	off := false
	policy.Save(ctx, vks.Scope{TenantID: tenant.ID, DomainID: domain.ID}, vks.Policy{Enabled: &off})
	if got := strings.TrimSpace(string(send("GET", bindings, token, "", 200))); got != "[]" {
		t.Fatalf("disabled VKS domain advertised Jitsi action: %s", got)
	}
	request("POST", apps+"/call", token, callBody, 403)
	request("GET", "/api/v1/vks/ice", token, "", 403)
	request("POST", "/api/v1/vks/join/"+room, token, "", 403)
}
