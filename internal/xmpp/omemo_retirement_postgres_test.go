package xmpp

import (
	"context"
	"database/sql"
	"encoding/xml"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestOMEMORetirementPostgres(t *testing.T) {
	raw := os.Getenv("TAYGA_TEST_OMEMO_POSTGRES_DSN")
	if raw == "" {
		t.Skip("set a disposable PostgreSQL DSN")
	}
	target, err := url.Parse(raw)
	if err != nil || (target.Scheme != "postgres" && target.Scheme != "postgresql") {
		t.Fatal("invalid fixture DSN")
	}
	db, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	schema := "omemo_test_" + strings.ReplaceAll(storage.NewID(), "-", "")
	if _, err = db.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Error(err)
		}
	}()
	q := target.Query()
	q.Set("search_path", schema)
	target.RawQuery = q.Encode()
	cfg := &config.Config{Storage: config.StorageConfig{Driver: "postgres", Postgres: config.PostgresConfig{DSN: target.String()}}}
	st, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tenant, err := st.CreateTenant(ctx, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := st.CreateDomain(ctx, tenant.ID, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	user, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "alice@example.com", LocalPart: "alice", AuthSource: "local", Enabled: true, PasswordHash: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	server := New(cfg, nil, st, nil, nil)
	other := New(cfg, nil, st, nil, nil)
	list, bundle := legacyOMEMO+".devicelist", legacyOMEMO+".bundles:42"
	if err = server.PublishWebOMEMO(ctx, user.Email, list, retirementList); err != nil {
		t.Fatal(err)
	}
	if err = server.PublishWebOMEMO(ctx, user.Email, bundle, retirementBundle); err != nil {
		t.Fatal(err)
	}
	// Force a write failure after the tombstone and bundle deletion; all must roll back.
	if _, err = db.Exec(`CREATE FUNCTION ` + schema + `.reject_list() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture failure'; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER reject_list BEFORE UPDATE ON ` + schema + `.xmpp_pep_items FOR EACH ROW EXECUTE FUNCTION ` + schema + `.reject_list()`); err != nil {
		t.Fatal(err)
	}
	if err = server.RetireWebOMEMO(ctx, user.Email, 42, "fixture-key"); err == nil {
		t.Fatal("injected error ignored")
	}
	retired, err := server.RetiredWebOMEMO(ctx, user.Email)
	if err != nil || len(retired) != 0 {
		t.Fatal(retired, err)
	}
	items, err := server.ReadWebOMEMO(ctx, user.Email, bundle)
	if err != nil || len(items) != 1 {
		t.Fatal("bundle deletion escaped rollback", items, err)
	}
	if _, err = db.Exec(`DROP TRIGGER reject_list ON ` + schema + `.xmpp_pep_items`); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	failures := make(chan error, 3)
	wg.Add(3)
	for _, s := range []*Server{server, other} {
		go func(s *Server) {
			defer wg.Done()
			<-start
			for i := 0; i < 20; i++ {
				if e := s.PublishWebOMEMO(ctx, user.Email, list, retirementList); e != nil {
					failures <- e
					return
				}
			}
		}(s)
	}
	go func() {
		defer wg.Done()
		<-start
		failures <- server.RetireWebOMEMO(ctx, user.Email, 42, "fixture-key")
	}()
	close(start)
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	items, err = other.ReadWebOMEMO(ctx, user.Email, list)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	var current omemoDeviceList
	if err = xml.Unmarshal([]byte(items[0].XML), &current); err != nil || len(current.Devices) != 1 || current.Devices[0].ID != 7 {
		t.Fatal(current, err)
	}
	if err = other.PublishWebOMEMO(ctx, user.Email, bundle, retirementBundle); err != ErrOMEMORetired {
		t.Fatal(err)
	}
}
