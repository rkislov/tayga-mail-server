package mattermost

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/mmsettings"
)

type pushRoundTrip func(*http.Request) (*http.Response, error)

func (f pushRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func enablePush(t *testing.T, f *fixture) {
	t.Helper()
	on := true
	url := mmsettings.TPNSURL
	if e := f.api.policy.Save(context.Background(), mmsettings.Scope{}, mmsettings.Overrides{Push: &mmsettings.PushOverrides{ProxyURL: &url, Enabled: &on}}); e != nil {
		t.Fatal(e)
	}
}
func pushRegister(t *testing.T, f *fixture, user int, letter string) {
	t.Helper()
	f.call(t, "PUT", "/users/sessions/device", f.tokens[user], map[string]string{"device_id": "apple_rn-v2:" + strings.Repeat(letter, 32)}, 200)
}
func TestAutomaticPushPrivacyRetriesAndDurability(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	enablePush(t, f)
	pushRegister(t, f, 0, "a")
	pushRegister(t, f, 1, "b")
	pushRegister(t, f, 3, "c")
	dm := f.call(t, "POST", "/channels/direct", f.tokens[0], []string{chat.ID(f.users[0].ID), chat.ID(f.users[1].ID)}, 201)
	body := map[string]string{"channel_id": dm["id"].(string), "message": "Private secret alice@example.com Alice", "pending_post_id": "push-retry-1"}
	p := f.call(t, "POST", "/posts", f.tokens[0], body, 201)
	f.call(t, "POST", "/posts", f.tokens[0], body, 201)
	var count int
	if e := f.store.DB().QueryRow(`SELECT COUNT(*) FROM chat_push_jobs`).Scan(&count); e != nil || count != 1 {
		t.Fatal("outbox dedup/recipient isolation", count, e)
	}
	var got []map[string]any
	f.api.pushClient = &http.Client{Transport: pushRoundTrip(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "Private") || strings.Contains(string(raw), "alice@example.com") || strings.Contains(string(raw), "Alice") || strings.Contains(string(raw), f.tokens[1]) {
			t.Fatal("private data left server")
		}
		var v map[string]any
		_ = json.Unmarshal(raw, &v)
		got = append(got, v)
		if v["message"] != "Новое сообщение" || v["platform"] != "apple_rn-v2" || v["device_id"] != strings.Repeat("b", 32) || v["post_id"] != p["id"] {
			t.Fatal(v)
		}
		if len(got) == 1 {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("temporary"))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"OK"}`))}, nil
	})}
	now := time.Now().Add(10 * time.Second).UnixMilli()
	if e := f.api.deliverPushBatch(ctx, now); e != nil {
		t.Fatal(e)
	}
	if e := f.api.deliverPushBatch(ctx, now+1000); e != nil {
		t.Fatal(e)
	}
	if len(got) != 1 {
		t.Fatal("retry delay not respected")
	}
	// Recreate the adapter: the outbox survives process-local state loss.
	restored := New(f.store, f.api.auth, f.api.log, "")
	restored.pushClient = f.api.pushClient
	if e := restored.deliverPushBatch(ctx, now+40000); e != nil {
		t.Fatal(e)
	}
	if e := restored.deliverPushBatch(ctx, now+80000); e != nil {
		t.Fatal(e)
	}
	if len(got) != 2 {
		t.Fatal("durable retry", len(got))
	}
}
func TestPushSuppressesReadDisabledRevokedAndForeignPolicy(t *testing.T) {
	for _, scenario := range []string{"read", "disabled", "logout", "policy", "preference"} {
		t.Run(scenario, func(t *testing.T) {
			f := setup(t)
			ctx := context.Background()
			enablePush(t, f)
			pushRegister(t, f, 1, "b")
			dm := f.call(t, "POST", "/channels/direct", f.tokens[0], []string{chat.ID(f.users[0].ID), chat.ID(f.users[1].ID)}, 201)
			f.call(t, "POST", "/posts", f.tokens[0], map[string]string{"channel_id": dm["id"].(string), "message": "hello"}, 201)
			switch scenario {
			case "read":
				f.call(t, "POST", "/channels/members/me/view", f.tokens[1], map[string]string{"channel_id": dm["id"].(string)}, 200)
			case "disabled":
				f.call(t, "PUT", "/users/sessions/device", f.tokens[1], map[string]string{"device_notification_disabled": "true"}, 200)
			case "logout":
				f.call(t, "POST", "/users/logout", f.tokens[1], nil, 200)
			case "policy":
				off := false
				if e := f.api.policy.Save(ctx, userScope(f.users[1]), mmsettings.Overrides{Push: &mmsettings.PushOverrides{Enabled: &off}}); e != nil {
					t.Fatal(e)
				}
			case "preference":
				f.call(t, "PUT", "/users/me/notify_props", f.tokens[1], map[string]string{"push": "none"}, 200)
			}
			f.api.pushClient = &http.Client{Transport: pushRoundTrip(func(r *http.Request) (*http.Response, error) {
				t.Error("suppressed notification reached gateway")
				return nil, fmt.Errorf("unexpected")
			})}
			if e := f.api.deliverPushBatch(ctx, time.Now().Add(10*time.Second).UnixMilli()); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestPushDeviceReassignmentAndTogglePreservesToken(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	pushRegister(t, f, 0, "x")
	session, _ := f.store.GetOAuthTokenByAccess(ctx, f.tokens[0])
	f.call(t, "PUT", "/users/sessions/device", f.tokens[0], map[string]string{"device_notification_disabled": "true"}, 200)
	d, e := f.api.chat.PushDevice(ctx, session.ID)
	if e != nil || !d.Disabled || d.DeviceID == "" {
		t.Fatal(d, e)
	}
	f.call(t, "PUT", "/users/sessions/device", f.tokens[0], map[string]string{"device_notification_disabled": "false"}, 200)
	pushRegister(t, f, 1, "x")
	if _, e = f.api.chat.PushDevice(ctx, session.ID); e == nil {
		t.Fatal("device still attached to previous account")
	}
}
func TestMentionMatchingAndScopedPushDiscovery(t *testing.T) {
	f := setup(t)
	u := f.users[0]
	if mentionsPush("@alice2", u) || mentionsPush("@alice.suffix", u) || !mentionsPush("Hi @alice!", u) || !mentionsPush("@channel", u) {
		t.Fatal("mention boundaries")
	}
	enablePush(t, f)
	if r := f.call(t, "GET", "/system/ping", f.tokens[0], nil, 200); r["CanReceiveNotifications"] != "true" {
		t.Fatal(r)
	}
	off := false
	if e := f.api.policy.Save(context.Background(), userScope(u), mmsettings.Overrides{Push: &mmsettings.PushOverrides{Enabled: &off}}); e != nil {
		t.Fatal(e)
	}
	if r := f.call(t, "GET", "/system/ping", f.tokens[0], nil, 200); r["CanReceiveNotifications"] != "false" {
		t.Fatal(r)
	}
}

func TestPushOutboxFailureRollsBackMessage(t *testing.T) {
	f := setup(t)
	pushRegister(t, f, 1, "b")
	dm := f.call(t, "POST", "/channels/direct", f.tokens[0], []string{chat.ID(f.users[0].ID), chat.ID(f.users[1].ID)}, 201)
	if _, e := f.store.DB().Exec(`CREATE TRIGGER reject_push BEFORE INSERT ON chat_push_jobs BEGIN SELECT RAISE(ABORT,'injected outbox failure'); END`); e != nil {
		t.Fatal(e)
	}
	body := map[string]string{"channel_id": dm["id"].(string), "message": "atomic", "pending_post_id": "atomic-1"}
	f.call(t, "POST", "/posts", f.tokens[0], body, 500)
	var count int
	if e := f.store.DB().QueryRow(`SELECT COUNT(*) FROM chat_posts`).Scan(&count); e != nil || count != 0 {
		t.Fatal("post committed without outbox", count, e)
	}
	if _, e := f.store.DB().Exec(`DROP TRIGGER reject_push`); e != nil {
		t.Fatal(e)
	}
	f.call(t, "POST", "/posts", f.tokens[0], body, 201)
}
func TestPushInvalidDeviceIsRemoved(t *testing.T) {
	f := setup(t)
	enablePush(t, f)
	pushRegister(t, f, 1, "b")
	dm := f.call(t, "POST", "/channels/direct", f.tokens[0], []string{chat.ID(f.users[0].ID), chat.ID(f.users[1].ID)}, 201)
	f.call(t, "POST", "/posts", f.tokens[0], map[string]string{"channel_id": dm["id"].(string), "message": "device gone"}, 201)
	f.api.pushClient = &http.Client{Transport: pushRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"REMOVE"}`))}, nil
	})}
	if e := f.api.deliverPushBatch(context.Background(), time.Now().Add(10*time.Second).UnixMilli()); e != nil {
		t.Fatal(e)
	}
	session, _ := f.store.GetOAuthTokenByAccess(context.Background(), f.tokens[1])
	if _, e := f.api.chat.PushDevice(context.Background(), session.ID); e == nil {
		t.Fatal("invalid device retained")
	}
}

func TestStagedPushRegistrationMigration(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	u := f.users[0]
	raw, _ := json.Marshal(map[string]any{"user_id": u.ID, "tenant_id": u.TenantID, "domain_id": u.DomainID, "device_id": "apple_rn-v2:" + strings.Repeat("a", 32), "disabled": false})
	if e := f.store.PutSetting(ctx, pushDeviceKey(f.tokens[0]), string(raw)); e != nil {
		t.Fatal(e)
	}
	if e := f.store.PutSetting(ctx, "unrelated.setting", "preserved"); e != nil {
		t.Fatal(e)
	}
	if e := f.api.migrateStagedPush(ctx); e != nil {
		t.Fatal(e)
	}
	session, _ := f.store.GetOAuthTokenByAccess(ctx, f.tokens[0])
	d, e := f.api.chat.PushDevice(ctx, session.ID)
	if e != nil || d.UserID != u.ID {
		t.Fatal(d, e)
	}
	if _, exists, e := f.store.GetSetting(ctx, pushDeviceKey(f.tokens[0])); e != nil || exists {
		t.Fatal("legacy registry retained", e)
	}
	if value, _, e := f.store.GetSetting(ctx, "unrelated.setting"); e != nil || value != "preserved" {
		t.Fatal("unrelated setting changed")
	}
}

func TestDomainPushDiscoveryDoesNotEnableOtherScopes(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	on := true
	proxy := mmsettings.TPNSURL
	if err := f.api.policy.Save(ctx, mmsettings.Scope{}, mmsettings.Overrides{Push: &mmsettings.PushOverrides{ProxyURL: &proxy}}); err != nil {
		t.Fatal(err)
	}
	check := func(token, want string) {
		t.Helper()
		if got := f.call(t, "GET", "/system/ping", token, nil, 200)["CanReceiveNotifications"]; got != want {
			t.Fatalf("got %v, want %s", got, want)
		}
		if got := f.call(t, "GET", "/config/client", token, nil, 200)["SendPushNotifications"]; got != want {
			t.Fatalf("client config got %v, want %s", got, want)
		}
	}
	check("", "false")
	if err := f.api.policy.Save(ctx, userScope(f.users[0]), mmsettings.Overrides{Push: &mmsettings.PushOverrides{Enabled: &on}}); err != nil {
		t.Fatal(err)
	}
	check("", "true")
	check(f.tokens[0], "true")
	check(f.tokens[3], "false")
	check("invalid-token", "false")
	off := false
	if err := f.api.policy.Save(ctx, userScope(f.users[0]), mmsettings.Overrides{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	check("", "false")
}
