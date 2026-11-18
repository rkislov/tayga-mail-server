package mattermost

import (
	"context"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/mmsettings"
)

func TestGenericPushPayloadPrivacy(t *testing.T) {
	f := setup(t)
	enablePush(t, f)
	pushRegister(t, f, 0, "a")
	calls := 0
	f.api.pushClient = &http.Client{Transport: pushRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		var payload map[string]any
		if e := json.NewDecoder(r.Body).Decode(&payload); e != nil {
			t.Fatal(e)
		}
		allowed := map[string]bool{"platform": true, "device_id": true, "server_id": true, "type": true, "message": true, "version": true, "cont_ava": true, "is_id_loaded": true, "badge": true, "ack_id": true, "signature": true}
		for k := range payload {
			if !allowed[k] {
				t.Error("unexpected field", k)
			}
		}
		if payload["message"] != "Тестовое уведомление TMS" || payload["platform"] != "apple_rn-v2" || payload["type"] != "message" {
			t.Fatal(payload["platform"], payload["type"])
		}
		key, e := f.api.pushKey(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		token, e := jwt.Parse(payload["signature"].(string), func(*jwt.Token) (any, error) { return &key.PublicKey, nil }, jwt.WithValidMethods([]string{"ES256"}))
		if e != nil || !token.Valid {
			t.Fatal("invalid push signature", e)
		}
		claims := token.Claims.(jwt.MapClaims)
		if claims["device_id"] != strings.Repeat("a", 32) || claims["ack_id"] != payload["ack_id"] || len(claims) != 2 {
			t.Fatal("incorrect signed claims")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"OK"}`))}, nil
	})}
	result := f.call(t, "POST", "/notifications/test", f.tokens[0], map[string]bool{"visible": true}, 200)
	if result["visible"] != true || result["gateway_accepted"] != true || result["ack_id"] == "" {
		t.Fatal(result)
	}
	f.call(t, "PUT", "/users/sessions/device", f.tokens[0], map[string]string{"device_id": "invalid"}, 400)
	if calls != 1 {
		t.Fatal("invalid device reached gateway")
	}
}

func TestPushPreparationAndSessionIsolation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	u := f.users[0]
	url := mmsettings.TPNSURL
	if err := f.api.policy.Save(ctx, mmsettings.Scope{}, mmsettings.Overrides{Push: &mmsettings.PushOverrides{ProxyURL: &url}}); err != nil {
		t.Fatal(err)
	}
	policy, err := f.api.policy.Resolve(ctx, userScope(u))
	if err != nil {
		t.Fatal(err)
	}
	if !policy.Push.Prepared || policy.Push.Enabled || policy.Push.IncludeMessageText {
		t.Fatal("preparation activated delivery or text")
	}
	bad := "http://127.0.0.1:8080"
	if err := f.api.policy.Save(ctx, mmsettings.Scope{}, mmsettings.Overrides{Push: &mmsettings.PushOverrides{ProxyURL: &bad}}); err == nil {
		t.Fatal("arbitrary gateway accepted")
	}
	f.call(t, "PUT", "/users/sessions/device", f.tokens[0], map[string]string{"device_id": "apple_rn:" + strings.Repeat("b", 32)}, 200)
	session, err := f.store.GetOAuthTokenByAccess(ctx, f.tokens[0])
	if err != nil {
		t.Fatal(err)
	}
	record, err := f.api.chat.PushDevice(ctx, session.ID)
	if err != nil {
		t.Fatal("device not saved", err)
	}
	raw, _ := json.Marshal(record)
	if strings.Contains(string(raw), f.tokens[0]) {
		t.Fatal("raw access token persisted")
	}
	f.call(t, "POST", "/notifications/test", f.tokens[1], nil, 409)
	f.call(t, "PUT", "/users/sessions/device", f.tokens[0], map[string]string{"device_id": "invalid"}, 400)
	f.call(t, "PUT", "/users/sessions/device", f.tokens[0], map[string]string{"device_id": ""}, 200)
	f.call(t, "POST", "/notifications/test", f.tokens[0], nil, 409)
}
