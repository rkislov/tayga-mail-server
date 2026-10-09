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
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/flowsync"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestFolderCreateAndMoveItems(t *testing.T) {
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
	inbox, err := store.EnsureMailbox(ctx, u.ID, "INBOX", ms.UserRoot(u.Email))
	if err != nil {
		t.Fatal(err)
	}
	msg, err := store.InsertMessage(ctx, &storage.Message{
		MailboxID: inbox.ID, Size: 5, Flags: "", FilePath: "a", InternalDate: time.Now().UTC(),
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

	createBody := `<FolderCreate><DisplayName>Archive</DisplayName><ParentId>0</ParentId><Type>12</Type></FolderCreate>`
	req := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=FolderCreate&DeviceId=d1&DeviceType=T", strings.NewReader(createBody))
	req.Header.Set("Authorization", authz)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	out := w.Body.String()
	if !strings.Contains(out, "<Status>1</Status>") || !strings.Contains(out, "<ServerId>") {
		t.Fatalf("create body: %s", out)
	}
	archive, err := store.GetMailbox(ctx, u.ID, "Archive")
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.ID) < 32 {
		t.Fatalf("expected UUID folder id %q", archive.ID)
	}

	moveBody := `<MoveItems><Move><SrcMsgId>` + msg.ID + `</SrcMsgId><DstFldId>` + archive.ID + `</DstFldId></Move></MoveItems>`
	req2 := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=MoveItems&DeviceId=d1&DeviceType=T", strings.NewReader(moveBody))
	req2.Header.Set("Authorization", authz)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("move %d %s", w2.Code, w2.Body.String())
	}
	got, err := store.GetMessageByID(ctx, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.MailboxID != archive.ID {
		t.Fatalf("message not moved: mailbox=%s want=%s", got.MailboxID, archive.ID)
	}
}
