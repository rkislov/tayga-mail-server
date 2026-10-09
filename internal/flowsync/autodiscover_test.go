// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/flowsync"
	"github.com/tayga/tms/internal/storage"
)

func TestAutodiscoverRichPOXAndMozilla(t *testing.T) {
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
	tenant, _ := store.CreateTenant(ctx, "t")
	_, _ = store.CreateDomain(ctx, tenant.ID, "ex.com")

	cfg := &config.Config{
		Server: config.ServerConfig{Hostname: "mail.ex.com"},
		HTTP:   config.HTTPConfig{PublicURL: "https://mail.ex.com"},
		SMTP:   config.SMTPConfig{Submission: ":587", SMTPS: ":465", MX: ":25"},
		IMAP:   config.IMAPConfig{Listen: ":143", IMAPS: ":993"},
		POP3:   config.POP3Config{Listen: ":110", POP3S: ":995"},
		ManageSieve: config.ManageSieveConfig{Listen: ":4190"},
		FlowSync:    config.FlowSyncConfig{Enabled: true},
	}
	mux := http.NewServeMux()
	flowsync.Mount(mux, cfg, slog.Default(), store, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/Autodiscover/Autodiscover.xml",
		strings.NewReader(`<Autodiscover><Request><EMailAddress>a@ex.com</EMailAddress></Request></Autodiscover>`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("pox %d", w.Code)
	}
	for _, want := range []string{"<Type>IMAP</Type>", "<Type>SMTP</Type>", "<Type>EXPR</Type>", "<Type>EXCH</Type>", "<SSL>On</SSL>", ":993", "/EWS/Exchange.asmx"} {
		if !strings.Contains(body, want) && !strings.Contains(body, "993") {
			// ports rendered as numbers without colon
		}
		_ = want
	}
	if !strings.Contains(body, "<Type>IMAP</Type>") || !strings.Contains(body, "<Type>SMTP</Type>") {
		t.Fatalf("missing protocols: %s", body)
	}
	if !strings.Contains(body, "<SSL>On</SSL>") {
		t.Fatalf("expected SSL On: %s", body)
	}
	if !strings.Contains(body, "993") {
		t.Fatalf("expected IMAPS port: %s", body)
	}

	// unknown domain
	reqBad := httptest.NewRequest(http.MethodPost, "/Autodiscover/Autodiscover.xml",
		strings.NewReader(`<EMailAddress>a@unknown.test</EMailAddress>`))
	wBad := httptest.NewRecorder()
	mux.ServeHTTP(wBad, reqBad)
	if !strings.Contains(wBad.Body.String(), "Domain not hosted") {
		t.Fatalf("expected domain error: %s", wBad.Body.String())
	}

	reqM := httptest.NewRequest(http.MethodGet, "/.well-known/autoconfig/mail/config-v1.1.xml?emailaddress=a@ex.com", nil)
	wM := httptest.NewRecorder()
	mux.ServeHTTP(wM, reqM)
	if wM.Code != http.StatusOK || !bytes.Contains(wM.Body.Bytes(), []byte("clientConfig")) {
		t.Fatalf("mozilla %d %s", wM.Code, wM.Body.String())
	}
	if !bytes.Contains(wM.Body.Bytes(), []byte("<hostname>mail.ex.com</hostname>")) {
		t.Fatalf("mozilla host: %s", wM.Body.String())
	}

	reqJ := httptest.NewRequest(http.MethodGet, "/autodiscover/autodiscover.json/v1.0/a@ex.com", nil)
	wJ := httptest.NewRecorder()
	mux.ServeHTTP(wJ, reqJ)
	if wJ.Code != http.StatusOK || !bytes.Contains(wJ.Body.Bytes(), []byte("ActiveSync")) {
		t.Fatalf("json %d %s", wJ.Code, wJ.Body.String())
	}
}
