package storage

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
)

func syncCompactFixture(t *testing.T, cfg config.StorageConfig) (*Store, *User, *User) {
	t.Helper()
	driver, err := Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { driver.Close() })
	s := driver.(*Store)
	tenant, _ := s.CreateTenant(t.Context(), "sync-compaction")
	domain, _ := s.CreateDomain(t.Context(), tenant.ID, "compact.test")
	makeUser := func(name string) *User {
		u, e := s.CreateUser(t.Context(), &User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: name, Email: name + "@compact.test", Enabled: true})
		if e != nil {
			t.Fatal(e)
		}
		return u
	}
	return s, makeUser("alice"), makeUser("bob")
}
func syncFixtureJSON(key string) string {
	items := map[string]string{}
	for i := 0; i < 1000; i++ {
		items[fmt.Sprintf("item-%04d", i)] = strings.Repeat("a", 64)
	}
	data, _ := json.Marshal(map[string]any{"Key": key, "PreviousKey": "unchanged-replay-key", "Items": items, "Response": "<Response><SyncState>" + key + "</SyncState>" + strings.Repeat("EXACT XML ", 500) + "</Response>"})
	return " \n" + string(data) + " \n"
}

func TestSyncStateDeduplicationIsLosslessAndScoped(t *testing.T) {
	configs := []config.StorageConfig{{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "sync.db")}}}
	if pg := bookingPostgresConfig(t); pg != nil {
		configs = append(configs, *pg)
	}
	for _, cfg := range configs {
		t.Run(cfg.Driver, func(t *testing.T) {
			s, alice, bob := syncCompactFixture(t, cfg)
			ctx := t.Context()
			for i := 0; i < 20; i++ {
				raw := syncFixtureJSON(fmt.Sprintf("client-key-%036d", i))
				client := fmt.Sprintf("ews:%d", i)
				if err := s.PutFlowSyncState(ctx, alice.ID, client, "inbox", raw); err != nil {
					t.Fatal(err)
				}
				got, err := s.GetFlowSyncState(ctx, alice.ID, client, "inbox")
				if err != nil || got != raw {
					t.Fatalf("replay bytes changed: %v", err)
				}
			}
			var count int
			if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT count(DISTINCT snapshot_id) FROM flowsync_states WHERE user_id=?`), alice.ID).Scan(&count); err != nil || count != 1 {
				t.Fatalf("20 identical maps not shared: %d %v", count, err)
			}
			if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT count(DISTINCT response_id) FROM flowsync_states WHERE user_id=?`), alice.ID).Scan(&count); err != nil || count != 1 {
				t.Fatal("replay templates not shared")
			}
			if err := s.PutFlowSyncState(ctx, bob.ID, "ews:1", "inbox", syncFixtureJSON("bob")); err != nil {
				t.Fatal(err)
			}
			if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM flowsync_state_blobs`).Scan(&count); err != nil || count != 4 {
				t.Fatal("snapshot scope crossed users")
			}
			// A wrong scope/reference cannot cause the reader to disclose another blob.
			var id string
			if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT snapshot_id FROM flowsync_states WHERE user_id=? AND client_id='ews:1'`), alice.ID).Scan(&id); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.ExecContext(ctx, s.rebind(`UPDATE flowsync_states SET snapshot_id=? WHERE user_id=? AND client_id='ews:1'`), id, bob.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.GetFlowSyncState(ctx, bob.ID, "ews:1", "inbox"); err == nil {
				t.Fatal("foreign snapshot disclosed")
			}
		})
	}
}

func TestSyncLegacyCompactionKeepsAgeAndReceipts(t *testing.T) {
	cfg := config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "legacy.db")}}
	s, alice, _ := syncCompactFixture(t, cfg)
	ctx := t.Context()
	raw := syncFixtureJSON("active-client-key")
	old := time.Now().UTC().Add(-45 * 24 * time.Hour)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO flowsync_states(user_id,client_id,collection_id,state,updated_at) VALUES(?,?,?,?,?)`, alice.ID, "ews:legacy", "inbox", raw, old); err != nil {
		t.Fatal(err)
	}
	if err := s.PutFlowSyncState(ctx, alice.ID, "submission", "receipt", "fingerprint-sentinel"); err != nil {
		t.Fatal(err)
	}
	report, err := s.CompactSyncStates(ctx, nil)
	if err != nil || !report.Verified || report.Compacted != 1 {
		t.Fatalf("compaction failed: %#v %v", report, err)
	}
	if report.TemplateBytes+report.SnapshotBytes >= report.LogicalBytes/2 {
		t.Fatal("state did not compact")
	}
	var updated time.Time
	if err = s.db.QueryRowContext(ctx, `SELECT updated_at FROM flowsync_states WHERE client_id='ews:legacy'`).Scan(&updated); err != nil || !updated.Equal(old) {
		t.Fatal("compaction changed client activity")
	}
	got, err := s.GetFlowSyncState(ctx, alice.ID, "ews:legacy", "inbox")
	if err != nil || got != raw {
		t.Fatal("client state changed")
	}
	got, err = s.GetFlowSyncState(ctx, alice.ID, "submission", "receipt")
	if err != nil || got != "fingerprint-sentinel" {
		t.Fatal("submission receipt changed")
	}
}

func TestSyncRetentionPreservesActiveClientsAndIdempotency(t *testing.T) {
	s, alice, _ := syncCompactFixture(t, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "retention.db")}})
	ctx := t.Context()
	old := time.Now().UTC().Add(-200 * 24 * time.Hour)
	for _, client := range []string{"ews:expired", "ews:active", "submission", "ews-tasks", "eas-device"} {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO flowsync_states(user_id,client_id,collection_id,state,updated_at) VALUES(?,?,?,?,?)`, alice.ID, client, "fixture", "receipt", old); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.GetFlowSyncState(ctx, alice.ID, "ews:active", "fixture"); err != nil {
		t.Fatal(err)
	}
	expired, _, err := s.CleanupSyncStates(ctx, 180*24*time.Hour)
	if err != nil || expired != 1 {
		t.Fatalf("unexpected expiry: %d %v", expired, err)
	}
	for _, client := range []string{"ews:active", "submission", "ews-tasks", "eas-device"} {
		value, err := s.GetFlowSyncState(ctx, alice.ID, client, "fixture")
		if err != nil || value != "receipt" {
			t.Fatalf("active/idempotency record deleted: %s", client)
		}
	}
}

func TestDeviceRetirementBlocksReenrollmentWithoutRemoteWipe(t *testing.T) {
	s, alice, bob := syncCompactFixture(t, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "retire.db")}})
	ctx := t.Context()
	device, err := s.EnsureFlowSyncDevice(ctx, alice.ID, "old-android", "Android")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.PutFlowSyncState(ctx, alice.ID, device.ID, "inbox", syncFixtureJSON("old-key")); err != nil {
		t.Fatal(err)
	}
	if err = s.PutFlowSyncState(ctx, alice.ID, device.ID, "task-add:receipt", "task:preserve"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RetireFlowSyncDevice(ctx, bob.ID, device.ID); err == nil {
		t.Fatal("foreign user retired device")
	}
	removed, err := s.RetireFlowSyncDevice(ctx, alice.ID, device.ID)
	if err != nil || removed != 1 {
		t.Fatalf("retirement: %d %v", removed, err)
	}
	same, err := s.EnsureFlowSyncDevice(ctx, alice.ID, "old-android", "Android")
	if err != nil || same.ID != device.ID || !same.Blocked || same.RetiredAt == nil || same.WipeStatus == "pending" {
		t.Fatal("device was reenrolled or wipe sent")
	}
	if err = s.SetFlowSyncDeviceControl(ctx, device.ID, false, "none"); err == nil {
		t.Fatal("retired ID unblocked")
	}
	if err = s.RestoreFlowSyncDevice(ctx, bob.ID, device.ID); err == nil {
		t.Fatal("foreign restoration accepted")
	}
	if err = s.RestoreFlowSyncDevice(ctx, alice.ID, device.ID); err != nil {
		t.Fatal(err)
	}
	restored, err := s.EnsureFlowSyncDevice(ctx, alice.ID, "old-android", "Android")
	if err != nil || !restored.Blocked || restored.RetiredAt != nil || restored.WipeStatus != "none" || restored.PolicyKey != "0" || restored.Provisioned {
		t.Fatal("restoration granted access or resumed old wipe")
	}
	if err = s.SetFlowSyncDeviceControl(ctx, device.ID, false, "none"); err != nil {
		t.Fatal("explicit unblock failed")
	}
	receipt, err := s.GetFlowSyncState(ctx, alice.ID, device.ID, "task-add:receipt")
	if err != nil || receipt != "task:preserve" {
		t.Fatal("task idempotency lost")
	}
}

func TestPostgresOfflineMaintenanceExcludesLiveWriters(t *testing.T) {
	cfg := bookingPostgresConfig(t)
	if cfg == nil {
		t.Skip("set TAYGA_TEST_BOOKING_POSTGRES_DSN")
	}
	ctx := t.Context()
	live, err := Open(ctx, *cfg)
	if err != nil {
		t.Fatal(err)
	}
	if exclusive, err := OpenOffline(ctx, *cfg); err == nil {
		exclusive.Close()
		live.Close()
		t.Fatal("offline maintenance accepted live writer")
	}
	if err = live.Close(); err != nil {
		t.Fatal(err)
	}
	exclusive, err := OpenOffline(ctx, *cfg)
	if err != nil {
		t.Fatal(err)
	}
	if another, err := Open(ctx, *cfg); err == nil {
		another.Close()
		exclusive.Close()
		t.Fatal("live writer entered offline maintenance")
	}
	if err = exclusive.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSyncCodecPreservesFieldOrderAndKeySubstitution(t *testing.T) {
	key := "opaque-key-01234567890123456789"
	client := "ews:client-01234567890123456789"
	response := "<SyncState>" + client + "/" + key + "</SyncState>" + strings.Repeat(" XML exact ", 500)
	parts := []map[string]any{{"Key": key, "Items": map[string]string{}, "Response": response}, {"Key": key, "Response": response}}
	for _, part := range parts {
		data, _ := json.Marshal(part)
		raw := "\n " + string(data) + "\n"
		encoded, err := encodeSyncState("user", client, "collection", raw)
		if err != nil {
			t.Fatal(err)
		}
		if encoded.responseID == "" {
			t.Fatal("large replay not shared")
		}
		decoded, err := decodeSyncState("user", "collection", encoded.state, syncStateBlob{encoded.id, encoded.data, int64(encoded.rawBytes)}, syncStateBlob{encoded.responseID, encoded.responseData, int64(encoded.responseBytes)})
		if err != nil || decoded != raw {
			t.Fatal("replay key substitution changed JSON bytes")
		}
	}
}

func TestSyncSnapshotGCProtectsBothLiveReferences(t *testing.T) {
	s, alice, _ := syncCompactFixture(t, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "gc.db")}})
	ctx := t.Context()
	if err := s.PutFlowSyncState(ctx, alice.ID, "ews:gc", "inbox", syncFixtureJSON("long-live-key-0123456789")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE flowsync_state_blobs SET created_at=?`, time.Now().UTC().Add(-10*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	_, removed, err := s.CleanupSyncStates(ctx, 180*24*time.Hour)
	if err != nil || removed != 0 {
		t.Fatal("live Items or replay snapshot collected")
	}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM flowsync_states WHERE client_id='ews:gc'`); err != nil {
		t.Fatal(err)
	}
	_, removed, err = s.CleanupSyncStates(ctx, 180*24*time.Hour)
	if err != nil || removed != 2 {
		t.Fatalf("orphan snapshots not collected: %d %v", removed, err)
	}
}
