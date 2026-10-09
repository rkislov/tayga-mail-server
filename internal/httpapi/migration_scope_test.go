package httpapi_test

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestMigrationTargetRequiresAdministrationScope(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	layer := auth.NewLayer(st, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, err := layer.Hasher.Hash("scope-test-password")
	if err != nil {
		t.Fatal(err)
	}
	tenant, err := st.CreateTenant(ctx, "example")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := st.CreateDomain(ctx, tenant.ID, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateDomain(ctx, tenant.ID, "other.example.com")
	if err != nil {
		t.Fatal(err)
	}
	create := func(local, domainID, email string, roles string) *storage.User {
		t.Helper()
		u, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domainID, LocalPart: local, Email: email, Enabled: true, AuthSource: "local", PasswordHash: hash, Roles: roles})
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	admin := create("admin", domain.ID, "manager@example.com", "domain_admin")
	if err := st.SetDomainAdminDomains(ctx, admin.ID, []string{domain.ID}); err != nil {
		t.Fatal(err)
	}
	ordinary := create("user", domain.ID, "user@example.com", "")
	outside := create("other", other.ID, "user@other.example.com", "")
	h := httpapi.New(cfg, slog.Default(), st, layer, mailstore.New(t.TempDir()), nil, nil).Handler()
	for _, tc := range []struct {
		name          string
		actor, target *storage.User
		denied        bool
	}{
		{"ordinary cannot select another account", ordinary, admin, true},
		{"administrator cannot select another domain", admin, outside, true},
		{"administrator can select managed user", admin, ordinary, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			login, err := layer.LoginWithPassword(ctx, tc.actor.Email, "scope-test-password")
			if err != nil {
				t.Fatal(err)
			}
			for _, action := range []string{"", "/jobs", "/jobs/example/retry", "/jobs/example/cancel"} {
				method := "POST"
				if action == "" {
					method = "GET"
				}
				r := httptest.NewRequest(method, "/api/v1/migration"+action+"?target_user_id="+tc.target.ID, strings.NewReader("{}"))
				r.Header.Set("Authorization", "Bearer "+login.Tokens.AccessToken)
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				denied := strings.Contains(w.Body.String(), "target account not permitted")
				if denied != tc.denied {
					t.Fatalf("%s: status %d response %s", action, w.Code, w.Body.String())
				}
				if denied && w.Code != 403 {
					t.Fatalf("expected forbidden, got %d", w.Code)
				}
			}
		})
	}
}
