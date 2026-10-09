package httpapi_test

import (
	"context"
	"fmt"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// BenchmarkMailPage measures a real authenticated 50-row API response over
// synthetic mailboxes. It excludes TLS, antivirus and network latency.
func BenchmarkMailPage(b *testing.B) {
	for _, count := range []int{1000, 20000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			ctx := context.Background()
			dir := b.TempDir()
			cfg := config.Default()
			store, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
			if err != nil {
				b.Fatal(err)
			}
			defer store.Close()
			layer := auth.NewLayer(store, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
			tenant, _ := store.CreateTenant(ctx, "bench")
			domain, _ := store.CreateDomain(ctx, tenant.ID, "bench.test")
			user, err := store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "user@bench.test", LocalPart: "user", AuthSource: "local", Enabled: true})
			if err != nil {
				b.Fatal(err)
			}
			ms := mailstore.New(filepath.Join(dir, "mail"))
			os.MkdirAll(ms.Root, 0750)
			os.WriteFile(filepath.Join(ms.Root, "sample"), []byte("From: sender@bench.test\r\nSubject: benchmark\r\nContent-Type: text/plain\r\n\r\nSynthetic body."), 0640)
			mailbox, err := store.EnsureMailbox(ctx, user.ID, "INBOX", ms.Root)
			if err != nil {
				b.Fatal(err)
			}
			for i := 0; i < count; i++ {
				_, err = store.InsertMessage(ctx, &storage.Message{MailboxID: mailbox.ID, UID: int64(i + 1), Size: 100, InternalDate: time.Now(), FilePath: "sample", MessageID: fmt.Sprintf("<%d@bench.test>", i), Subject: "benchmark", FromAddr: "sender@bench.test", ToAddr: user.Email})
				if err != nil {
					b.Fatal(err)
				}
			}
			tokens, err := layer.Tokens.IssueTokens(ctx, user.ID)
			if err != nil {
				b.Fatal(err)
			}
			handler := httpapi.New(cfg, slog.Default(), store, layer, ms, nil, nil).Handler()
			url := "/api/v1/mail/mailboxes/" + mailbox.ID + "/messages?limit=50"
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					request := httptest.NewRequest("GET", url, nil)
					request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
					response := httptest.NewRecorder()
					handler.ServeHTTP(response, request)
					if response.Code != 200 {
						b.Errorf("response %d", response.Code)
					}
				}
			})
		})
	}
}
