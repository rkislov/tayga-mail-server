// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
	"path/filepath"
	"strings"
	"testing"
)

func TestEWSCollectionAccessAndXML(t *testing.T) {
	ctx := context.Background()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "test.db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tenant, err := st.CreateTenant(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	dom, err := st.CreateDomain(ctx, tenant.ID, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	users := []*storage.User{}
	for _, name := range []string{"alice", "bob"} {
		u, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: dom.ID, Email: name + "@example.com", LocalPart: name, AuthSource: "local", Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, u)
	}
	mb, err := st.EnsureMailbox(ctx, users[1].ID, "INBOX", "/unused")
	if err != nil {
		t.Fatal(err)
	}
	h := &ewsHandler{store: st}
	for _, method := range []func(context.Context, *storage.User, string) (string, error){h.findItem, h.syncFolderItems} {
		result, err := method(ctx, users[0], `<Request><FolderId Id="`+mb.ID+`"/></Request>`)
		if err != nil || !strings.Contains(result, "ErrorAccessDenied") {
			t.Fatalf("cross-user read accepted: %s %v", result, err)
		}
	}
	result, err := h.findMailItems(ctx, users[1], mb.ID)
	if err != nil {
		t.Fatal(err)
	}
	requireXML(t, result)
}
