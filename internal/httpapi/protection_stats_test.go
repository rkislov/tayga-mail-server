package httpapi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestProtectionStatisticsAuthorizationAndScope(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	f.call(0, "GET", "/api/v1/admin/protection/stats", "", 403)
	u, _ := f.s.store.GetUserByID(ctx, f.users[0])
	if err := f.s.store.UpdateUserRoles(ctx, u.ID, "tenant_admin"); err != nil {
		t.Fatal(err)
	}
	st := f.s.store.(*storage.Store)
	other, _ := st.CreateTenant(ctx, "foreign")
	domain, _ := st.CreateDomain(ctx, other.ID, "foreign.example")
	for _, d := range []string{u.DomainID, domain.ID} {
		if err := st.RecordProtection(ctx, d, "learning", "spam"); err != nil {
			t.Fatal(err)
		}
	}
	var result struct {
		Counts map[string]map[string]int64 `json:"counts"`
		Rspamd any                         `json:"rspamd"`
	}
	json.Unmarshal(f.call(0, "GET", "/api/v1/admin/protection/stats", "", 200).Body.Bytes(), &result)
	if result.Counts["learning"]["spam"] != 1 || result.Rspamd != nil {
		t.Fatal(result)
	}
}
