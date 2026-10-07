package flowsync_test

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/flowsync"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestFlowSyncWritebackCalendarContactsMail(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	defer os.RemoveAll(dir)

	ms := mailstore.New(filepath.Join(dir, "mail"))
	layer := auth.NewLayer(store, config.LDAPConfig{}, config.MFAConfig{
		Issuer: "T", AccessTokenTTL: time.Hour, RefreshTokenTTL: 24 * time.Hour, ChallengeTTL: 5 * time.Minute,
	}, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("secret")
	tenant, _ := store.CreateTenant(ctx, "t")
	dom, _ := store.CreateDomain(ctx, tenant.ID, "ex.com")
	u, err := store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local",
		PasswordHash: hash, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	mb, err := store.EnsureMailbox(ctx, u.ID, "INBOX", "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.EnsureDAVDefaults(ctx, u.ID)
	cal, _ := store.GetCalendarByName(ctx, u.ID, "default")
	ab, _ := store.GetAddressBookByName(ctx, u.ID, "default")

	msg, err := store.InsertMessage(ctx, &storage.Message{
		MailboxID: mb.ID, UID: 1, Size: 10, Flags: "", FilePath: "x", InternalDate: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Server:   config.ServerConfig{Hostname: "mail.ex.com"},
		HTTP:     config.HTTPConfig{PublicURL: "http://127.0.0.1:8080"},
		FlowSync: config.FlowSyncConfig{Enabled: true},
	}
	mux := http.NewServeMux()
	flowsync.Mount(mux, cfg, slog.Default(), store, layer, ms)
	authz := "Basic " + base64.StdEncoding.EncodeToString([]byte("u@ex.com:secret"))

	// Calendar Add via Sync
	calBody := `<Sync><Collections><Collection><Class>Calendar</Class><SyncKey>0</SyncKey><CollectionId>` + cal.ID + `</CollectionId>` +
		`<Commands><Add><ClientId>c1</ClientId><ApplicationData>` +
		`<Subject>Demo</Subject><Location>HQ</Location>` +
		`<StartTime>2026-10-08T09:00:00.000Z</StartTime><EndTime>2026-10-08T10:00:00.000Z</EndTime>` +
		`</ApplicationData></Add></Commands></Collection></Collections></Sync>`
	req := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId=d1&DeviceType=T", strings.NewReader(calBody))
	req.Header.Set("Authorization", authz)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cal add %d %s", w.Code, w.Body.String())
	}
	out := w.Body.String()
	if !strings.Contains(out, "<ClientId>c1</ClientId>") || !strings.Contains(out, "<ServerId>") {
		t.Fatalf("missing writeback response: %s", out)
	}
	objs, _ := store.ListCalendarObjects(ctx, cal.ID)
	if len(objs) != 1 || !strings.Contains(objs[0].Data, "Demo") {
		t.Fatalf("calendar not persisted: %+v", objs)
	}
	calObjID := objs[0].ID

	// Contacts Add
	cardBody := `<Sync><Collections><Collection><Class>Contacts</Class><SyncKey>0</SyncKey><CollectionId>` + ab.ID + `</CollectionId>` +
		`<Commands><Add><ClientId>p1</ClientId><ApplicationData>` +
		`<FileAs>Grace Hopper</FileAs><FirstName>Grace</FirstName><LastName>Hopper</LastName>` +
		`<Email1Address>grace@ex.com</Email1Address>` +
		`</ApplicationData></Add></Commands></Collection></Collections></Sync>`
	req2 := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId=d1&DeviceType=T", strings.NewReader(cardBody))
	req2.Header.Set("Authorization", authz)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("contact add %d %s", w2.Code, w2.Body.String())
	}
	cards, _ := store.ListAddressObjects(ctx, ab.ID)
	if len(cards) != 1 || !strings.Contains(cards[0].Data, "Grace Hopper") {
		t.Fatalf("contact not persisted: %+v", cards)
	}

	// Mail Change Read=1
	mailBody := `<Sync><Collections><Collection><SyncKey>0</SyncKey><CollectionId>` + mb.ID + `</CollectionId>` +
		`<Commands><Change><ServerId>` + msg.ID + `</ServerId><ApplicationData><Read>1</Read></ApplicationData></Change></Commands>` +
		`</Collection></Collections></Sync>`
	req3 := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId=d1&DeviceType=T", strings.NewReader(mailBody))
	req3.Header.Set("Authorization", authz)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("mail change %d %s", w3.Code, w3.Body.String())
	}
	got, _ := store.GetMessageByID(ctx, msg.ID)
	if !storage.HasFlag(got.Flags, `\Seen`) {
		t.Fatalf("expected \\Seen, flags=%q", got.Flags)
	}

	// Calendar Delete
	delBody := `<Sync><Collections><Collection><Class>Calendar</Class><SyncKey>1</SyncKey><CollectionId>` + cal.ID + `</CollectionId>` +
		`<Commands><Delete><ServerId>` + calObjID + `</ServerId></Delete></Commands></Collection></Collections></Sync>`
	req4 := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId=d1&DeviceType=T", strings.NewReader(delBody))
	req4.Header.Set("Authorization", authz)
	w4 := httptest.NewRecorder()
	mux.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("cal del %d %s", w4.Code, w4.Body.String())
	}
	objs, _ = store.ListCalendarObjects(ctx, cal.ID)
	if len(objs) != 0 {
		t.Fatalf("expected calendar empty after delete, got %d", len(objs))
	}

	// EWS CreateItem Contact
	ewsBody := `<CreateItem><SavedItemFolderId><FolderId Id="` + ab.ID + `"/></SavedItemFolderId>` +
		`<Items><Contact><DisplayName>Alan Turing</DisplayName><GivenName>Alan</GivenName><Surname>Turing</Surname>` +
		`<EmailAddresses><Entry>alan@ex.com</Entry></EmailAddresses></Contact></Items></CreateItem>`
	req5 := httptest.NewRequest(http.MethodPost, "/EWS/Exchange.asmx", strings.NewReader(ewsBody))
	req5.Header.Set("Authorization", authz)
	w5 := httptest.NewRecorder()
	mux.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Fatalf("ews create %d %s", w5.Code, w5.Body.String())
	}
	if !strings.Contains(w5.Body.String(), "ItemId") {
		t.Fatalf("ews create missing ItemId: %s", w5.Body.String())
	}
	cards, _ = store.ListAddressObjects(ctx, ab.ID)
	if len(cards) < 2 {
		t.Fatalf("expected ews contact persisted, got %d", len(cards))
	}
}
