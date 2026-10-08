package settings

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestConfigSchemaCoversEveryField(t *testing.T) {
	schema := ConfigSchema()
	typ := reflect.TypeOf(config.Config{})
	if len(schema) != typ.NumField() {
		t.Fatalf("schema has %d sections, config has %d", len(schema), typ.NumField())
	}
	var check func(FieldSchema)
	count := 0
	check = func(field FieldSchema) {
		if field.Label == "" || field.Description == "" {
			t.Errorf("missing help for %s", field.Key)
		}
		if field.Kind == "duration" {
			if _, ok := field.Default.(string); !ok {
				t.Errorf("duration %s has non-string default", field.Key)
			}
		}
		if field.Secret && field.Default != "" {
			t.Errorf("secret default exposed: %s", field.Key)
		}
		for _, child := range field.Fields {
			check(child)
		}
		if field.Item != nil {
			check(*field.Item)
		}
		count++
	}
	for _, section := range schema {
		for _, child := range section.Fields {
			check(child)
		}
	}
	if count < 200 {
		t.Fatalf("unexpectedly small schema: %d fields", count)
	}
	if !schema["storage"].ReadOnly {
		t.Fatal("bootstrap storage is editable")
	}
	if !schema["mailstore"].Fields[0].ReadOnly {
		t.Fatal("maildir root is editable")
	}
	for _, section := range EditableSections {
		if _, ok := schema[section]; !ok {
			t.Errorf("missing editable section %s", section)
		}
	}
}

func TestCatalogRedactsAndPreservesSecrets(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	cfg := config.Default()
	cfg.Storage.SQLite.Path = filepath.Join(dir, "t.db")
	cfg.Mailstore.Root = filepath.Join(dir, "mail")
	cfg.Server.SecretsKey = "server-encryption-secret"
	cfg.Seed.Password = "seed-secret"
	cfg.Storage.Postgres.DSN = "postgres://user:private@localhost/mail"
	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hub := NewHub(store, cfg)
	if err := hub.Load(ctx); err != nil {
		t.Fatal(err)
	}
	out, err := hub.ListAll()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(out)
	var values map[string]any
	if err := json.Unmarshal(b, &values); err != nil {
		t.Fatal(err)
	}
	sections := values["settings"].(map[string]any)
	if sections["server"].(map[string]any)["secrets_key"] != redacted {
		t.Fatal("server key exposed")
	}
	if sections["seed"].(map[string]any)["password"] != redacted {
		t.Fatal("seed password exposed")
	}
	if sections["storage"].(map[string]any)["postgres"].(map[string]any)["dsn"] != redacted {
		t.Fatal("bootstrap DSN exposed")
	}
	if err := hub.PutSection(ctx, "server", []byte(`{"hostname":"new.example.com","secrets_key":"***"}`)); err != nil {
		t.Fatal(err)
	}
	if hub.Config().Server.SecretsKey != "server-encryption-secret" {
		t.Fatal("masked server key overwritten")
	}
	if err := hub.PutSection(ctx, "seed", []byte(`{"enabled":false,"password":""}`)); err != nil {
		t.Fatal(err)
	}
	if hub.Config().Seed.Password != "seed-secret" {
		t.Fatal("blank seed password overwritten")
	}
	if err := hub.PutSection(ctx, "storage", []byte(`{"driver":"postgres"}`)); err == nil {
		t.Fatal("bootstrap storage accepted for editing")
	}
}

func TestComponentSecretsFollowIdentity(t *testing.T) {
	old := config.Default()
	old.XMPP.Components = []config.XMPPComponentConfig{{Name: "first", Subdomain: "one", Secret: "one-secret"}, {Name: "second", Subdomain: "two", Secret: "two-secret"}}
	next := cloneConfig(old)
	next.XMPP.Components = []config.XMPPComponentConfig{{Name: "second", Subdomain: "two", Secret: redacted}}
	preserveSecrets(old, next)
	if next.XMPP.Components[0].Secret != "two-secret" {
		t.Fatal("removing a component transferred another secret")
	}
}
