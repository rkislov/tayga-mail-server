// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync_test

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/flowsync"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestFlowSyncCalendarAndContactsUUID(t *testing.T) {
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
	if _, err := store.EnsureMailbox(ctx, u.ID, "INBOX", "/tmp"); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureDAVDefaults(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	cal, err := store.GetCalendarByName(ctx, u.ID, "default")
	if err != nil {
		t.Fatal(err)
	}
	ab, err := store.GetAddressBookByName(ctx, u.ID, "default")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	calObj, err := store.UpsertCalendarObject(ctx, &storage.CalendarObject{
		CalendarID: cal.ID,
		UID:        "evt-1",
		HrefName:   "evt-1.ics",
		Component:  "VEVENT",
		DTStart:    &start,
		DTEnd:      &end,
		Data: `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:evt-1
SUMMARY:Standup
LOCATION:Zoom
DTSTART:20261007T100000Z
DTEND:20261007T110000Z
END:VEVENT
END:VCALENDAR`,
	})
	if err != nil {
		t.Fatal(err)
	}
	cardObj, err := store.UpsertAddressObject(ctx, &storage.AddressObject{
		AddressBookID: ab.ID,
		UID:           "card-1",
		HrefName:      "card-1.vcf",
		Data: `BEGIN:VCARD
VERSION:3.0
FN:Ada Lovelace
N:Lovelace;Ada;;;
EMAIL:ada@ex.com
TEL:+1000
END:VCARD`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cal.ID) < 32 || len(calObj.ID) < 32 || len(cardObj.ID) < 32 {
		t.Fatalf("expected UUIDs cal=%q obj=%q card=%q", cal.ID, calObj.ID, cardObj.ID)
	}

	cfg := &config.Config{
		Server:   config.ServerConfig{Hostname: "mail.ex.com"},
		HTTP:     config.HTTPConfig{Listen: ":8080", PublicURL: "http://127.0.0.1:8080"},
		FlowSync: config.FlowSyncConfig{Enabled: true},
	}
	mux := http.NewServeMux()
	flowsync.Mount(mux, cfg, slog.Default(), store, layer, ms)
	authz := "Basic " + base64.StdEncoding.EncodeToString([]byte("u@ex.com:secret"))

	// FolderSync includes calendar + contacts UUIDs
	req := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=FolderSync&DeviceId=d1&DeviceType=Test", nil)
	req.Header.Set("Authorization", authz)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("foldersync %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, cal.ID) || !strings.Contains(body, ab.ID) {
		t.Fatalf("FolderSync missing cal/ab UUIDs: %s", body)
	}
	if !strings.Contains(body, `<Type>8</Type>`) || !strings.Contains(body, `<Type>9</Type>`) {
		t.Fatalf("expected calendar/contacts folder types: %s", body)
	}

	// Sync Calendar collection
	syncBody := `<Sync><Collections><Collection><Class>Calendar</Class><SyncKey>0</SyncKey><CollectionId>` + cal.ID + `</CollectionId></Collection></Collections></Sync>`
	req2 := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId=d1&DeviceType=Test", strings.NewReader(syncBody))
	req2.Header.Set("Authorization", authz)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("sync cal %d %s", w2.Code, w2.Body.String())
	}
	out := w2.Body.String()
	if !strings.Contains(out, calObj.ID) || !strings.Contains(out, "Standup") {
		t.Fatalf("calendar sync missing event UUID/subject: %s", out)
	}

	// Sync Contacts
	syncCard := `<Sync><Collections><Collection><Class>Contacts</Class><SyncKey>0</SyncKey><CollectionId>` + ab.ID + `</CollectionId></Collection></Collections></Sync>`
	req3 := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId=d1&DeviceType=Test", strings.NewReader(syncCard))
	req3.Header.Set("Authorization", authz)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("sync contacts %d %s", w3.Code, w3.Body.String())
	}
	cout := w3.Body.String()
	if !strings.Contains(cout, cardObj.ID) || !strings.Contains(cout, "Ada Lovelace") {
		t.Fatalf("contacts sync missing UUID/name: %s", cout)
	}

	// Provision includes policy data + UUID policy key
	legacyDevice, err := store.EnsureFlowSyncDevice(ctx, u.ID, "d1", "Test")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetFlowSyncPolicyKey(ctx, legacyDevice.ID, storage.NewID()); err != nil {
		t.Fatal(err)
	}
	req4 := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=Provision&DeviceId=d1&DeviceType=Test", nil)
	req4.Header.Set("Authorization", authz)
	w4 := httptest.NewRecorder()
	mux.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("provision %d", w4.Code)
	}
	pout := w4.Body.String()
	if !strings.Contains(pout, "DevicePasswordEnabled") || !strings.Contains(pout, "MinDevicePasswordLength") {
		t.Fatalf("provision missing policy data: %s", pout)
	}
	if !strings.Contains(pout, "<PolicyKey>") {
		t.Fatalf("provision missing PolicyKey: %s", pout)
	}
	policyKey := strings.Split(strings.Split(pout, "<PolicyKey>")[1], "</PolicyKey>")[0]
	if value, err := strconv.ParseUint(policyKey, 10, 32); err != nil || value == 0 {
		t.Fatalf("policy key must be a nonzero uint32, got %q", policyKey)
	}
	repeated := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=Provision&DeviceId=d1&DeviceType=Test", nil)
	repeated.Header.Set("Authorization", authz)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, repeated)
	if !strings.Contains(response.Body.String(), "<PolicyKey>"+policyKey+"</PolicyKey>") {
		t.Fatal("policy key changed during provisioning handshake")
	}

}
