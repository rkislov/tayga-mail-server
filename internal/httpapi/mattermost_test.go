package httpapi_test

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/storage"
)

func TestMattermostOptIn(t *testing.T) {
	cfg := config.Default()
	cfg.FlowSync.Enabled = false
	cfg.Storage.SQLite.Path = filepath.Join(t.TempDir(), "mattermost.db")
	st, err := storage.Open(context.Background(), cfg.Storage)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	layer := auth.NewLayer(st, cfg.LDAP, cfg.MFA, cfg.OIDC)
	for _, enabled := range []bool{false, true} {
		cfg.Mattermost.Enabled = enabled
		handler := httpapi.New(cfg, slog.Default(), st, layer, nil, nil, nil).Handler()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v4/system/ping", nil))
		want := 404
		if enabled {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("enabled=%v: %d %s", enabled, w.Code, w.Body.String())
		}
	}
}
