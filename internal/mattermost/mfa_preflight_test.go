package mattermost

import (
	"context"
	"testing"

	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
)

func TestMattChaUnauthenticatedMFAPreflight(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	u := f.users[0]
	check := func(login string, required bool) {
		t.Helper()
		v := f.call(t, "POST", "/users/mfa", "", map[string]string{"login_id": login}, 200)
		if v["mfa_required"] != required || len(v) != 1 {
			t.Fatal(v)
		}
	}
	check("  ALICE@EXAMPLE.COM ", false)
	check("missing@example.com", false)
	f.call(t, "POST", "/users/mfa", "", map[string]string{}, 400)
	if err := f.store.UpsertUserMFA(ctx, &storage.UserMFA{UserID: u.ID, TOTPEnabled: true, TOTPSecret: "JBSWY3DPEHPK3PXP", BackupCodes: "[]"}); err != nil {
		t.Fatal(err)
	}
	check(u.Email, true)
	// A successful discovery is not a session and does not bypass the second factor.
	f.call(t, "GET", "/users/me", "", nil, 401)
	f.call(t, "POST", "/users/login", "", map[string]string{"login_id": u.Email, "password": "secret"}, 401)
	f.call(t, "POST", "/users/login", "", map[string]string{"login_id": u.Email, "password": "wrong"}, 401)
	off := false
	if err := f.api.policy.Save(ctx, userScope(u), mmsettings.Overrides{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	check(u.Email, false)
}
