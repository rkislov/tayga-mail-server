// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func syncFixture(t *testing.T) (context.Context, storage.Driver, *storage.User, *easHandler, *storage.FlowSyncDevice, string) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "state.db")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	tenant, err := st.CreateTenant(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	dom, err := st.CreateDomain(ctx, tenant.ID, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: dom.ID, LocalPart: "alice", Email: "alice@example.com", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = st.EnsureDAVDefaults(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	ab, err := st.GetAddressBookByName(ctx, u.ID, "default")
	if err != nil {
		t.Fatal(err)
	}
	device, err := st.EnsureFlowSyncDevice(ctx, u.ID, "test", "test")
	if err != nil {
		t.Fatal(err)
	}
	return ctx, st, u, &easHandler{store: st, ms: mailstore.New(filepath.Join(dir, "mail"))}, device, ab.ID
}
func TestSyncWindowReplayChangesAndInvalidKey(t *testing.T) {
	ctx, st, u, h, dev, ab := syncFixture(t)
	var cards []*storage.AddressObject
	for _, name := range []string{"a", "b", "c"} {
		o, err := st.UpsertAddressObject(ctx, &storage.AddressObject{AddressBookID: ab, UID: name, HrefName: name + ".vcf", Data: "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:" + name + "\r\nEND:VCARD\r\n"})
		if err != nil {
			t.Fatal(err)
		}
		cards = append(cards, o)
	}
	request := func(key, commands string) string {
		return `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + ab + `</CollectionId><WindowSize>2</WindowSize>` + commands + `</Collection></Collections></Sync>`
	}
	call := func(body string) string {
		result, err := h.syncCollections(ctx, u, dev, []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		requireXML(t, result)
		return result
	}
	key := func(result string) string { return extractTag(result, "SyncKey") }
	initial := call(request("0", ""))
	if strings.Contains(initial, "ApplicationData") {
		t.Fatal("initial handshake included data")
	}
	firstRequest := request(key(initial), "")
	first := call(firstRequest)
	if !strings.Contains(first, "MoreAvailable") {
		t.Fatal("missing page continuation")
	}
	replay := call(firstRequest)
	if replay != first {
		t.Fatal("retry changed response")
	}
	second := call(request(key(first), ""))
	if strings.Contains(second, "MoreAvailable") {
		t.Fatal("unexpected continuation")
	}
	idle := call(request(key(second), ""))
	if strings.Contains(idle, "ApplicationData") {
		t.Fatal("unchanged items repeated")
	}
	cards[0].Data = "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:changed\r\nEND:VCARD\r\n"
	if _, err := st.UpsertAddressObject(ctx, cards[0]); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteAddressObject(ctx, ab, cards[1].HrefName); err != nil {
		t.Fatal(err)
	}
	changed := call(request(key(idle), ""))
	if !strings.Contains(changed, "<Change") || !strings.Contains(changed, "<Delete") {
		t.Fatal(changed)
	}
	bad := call(request("invented", `<Commands><Delete><ServerId>`+cards[2].ID+`</ServerId></Delete></Commands>`))
	if !strings.Contains(bad, "<Status>3</Status>") {
		t.Fatal(bad)
	}
	if _, err := st.GetAddressObjectByID(ctx, cards[2].ID); err != nil {
		t.Fatal("invalid key mutated data")
	}
	// State survives reconstruction of the protocol handler.
	h = &easHandler{store: st, ms: h.ms}
	idle = call(request(key(changed), ""))
	if strings.Contains(idle, "ApplicationData") {
		t.Fatal("restart lost sync state")
	}
}
func TestWireCodecAndCommandResponses(t *testing.T) {
	ctx, _, u, h, _, _ := syncFixture(t)
	for _, command := range []string{"FolderSync", "Provision", "GetItemEstimate"} {
		document := `<` + command + ` xmlns="` + command + `:"></` + command + `>`
		if command == "FolderSync" {
			document = `<FolderSync xmlns="FolderHierarchy:"><SyncKey>0</SyncKey></FolderSync>`
		}
		if command == "GetItemEstimate" {
			continue
		}
		body, err := encodeWire(document)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd="+command+"&DeviceId=wire", strings.NewReader(string(body))).WithContext(withUser(ctx, u))
		req.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("%s %d %s", command, w.Code, w.Body.String())
		}
		xmlBody, err := decodeWire(w.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		requireXML(t, string(xmlBody))
	}
	for _, bad := range [][]byte{{3, 1, 106, 0, 0, 7, 0x56}, {3, 1, 106, 0, 1}, {3, 1, 106, 0, 0xc3, 0x7f}, {3, 1, 106, 0, 5, 5}} {
		if _, err := decodeWire(bad); err == nil {
			t.Fatalf("accepted malformed wire: %x", bad)
		}
	}
}
func FuzzDecodeWire(f *testing.F) {
	f.Add([]byte{3, 1, 106, 0, 0, 7, 0x56, 0x52, 3, '0', 0, 1, 1})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		decoded, err := decodeWire(b)
		if err == nil {
			requireXML(t, string(decoded))
		}
	})
}
