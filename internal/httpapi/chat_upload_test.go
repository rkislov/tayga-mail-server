package httpapi_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// Exercise the capability URLs without a login cookie or bearer token, as
// native clients and a web client on a different origin download attachments.
func TestChatEncryptedUploadHTTPContract(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.HTTP.PublicURL = "https://mail.example.com"
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	layer := auth.NewLayer(st, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, err := layer.Hasher.Hash("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	tenant, err := st.CreateTenant(ctx, "upload-test")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := st.CreateDomain(ctx, tenant.ID, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "files@example.com", LocalPart: "files", Enabled: true, AuthSource: "local", PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	login, err := layer.LoginWithPassword(ctx, u.Email, "fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), st, layer, mailstore.New(filepath.Join(dir, "mail")), nil, nil).Handler()
	call := func(method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	key, iv, plain := make([]byte, 32), make([]byte, 12), []byte("private attachment fixture\x00\xff")
	if _, err = rand.Read(key); err != nil {
		t.Fatal(err)
	}
	if _, err = rand.Read(iv); err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	wire := gcm.Seal(nil, iv, plain, nil)
	body, _ := json.Marshal(map[string]any{"name": "fixture.pdf", "size": len(wire), "type": "application/octet-stream"})
	w := call("POST", "/api/v1/chat/upload", body, map[string]string{"Authorization": "Bearer " + login.Tokens.AccessToken})
	if w.Code != 201 {
		t.Fatalf("slot: %d %s", w.Code, w.Body.String())
	}
	var slot struct{ Put, Get string }
	if err = json.Unmarshal(w.Body.Bytes(), &slot); err != nil {
		t.Fatal(err)
	}
	put, _ := url.Parse(slot.Put)
	get, _ := url.Parse(slot.Get)
	origin := "https://web.other.example"
	for _, test := range []struct{ path, method string }{{put.RequestURI(), "PUT"}, {get.RequestURI(), "GET"}} {
		w = call("OPTIONS", test.path, nil, map[string]string{"Origin": origin, "Access-Control-Request-Method": test.method, "Access-Control-Request-Headers": "content-type"})
		if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "*" || !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), test.method) {
			t.Errorf("preflight %s: %d %v", test.method, w.Code, w.Header())
		}
	}
	w = call("PUT", put.RequestURI(), wire[:len(wire)-1], nil)
	if w.Code != 400 {
		t.Fatalf("truncated upload accepted: %d", w.Code)
	}
	w = call("PUT", put.RequestURI(), wire, map[string]string{"Content-Type": "application/octet-stream", "Origin": origin})
	if w.Code != 201 {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("upload response not available cross-origin")
	}
	token := strings.Split(get.Path, "/")[2]
	share, err := st.GetFileShareByToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		w = call("HEAD", get.RequestURI(), nil, map[string]string{"Origin": origin})
		if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Length") != strconv.Itoa(len(wire)) {
			t.Errorf("HEAD: %d length=%s body=%d", w.Code, w.Header().Get("Content-Length"), w.Body.Len())
		}
	}
	afterHead, err := st.GetFileShareByToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if afterHead.DownloadCount != share.DownloadCount {
		t.Errorf("HEAD consumed downloads: %d -> %d", share.DownloadCount, afterHead.DownloadCount)
	}
	w = call("GET", get.RequestURI(), nil, map[string]string{"Origin": origin, "Range": "bytes=0-15"})
	if w.Code != 206 || !bytes.Equal(w.Body.Bytes(), wire[:16]) {
		t.Fatal("range download changed ciphertext")
	}
	w = call("GET", get.RequestURI(), nil, map[string]string{"Origin": origin})
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), wire) {
		t.Fatal("download changed ciphertext")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" || w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Error("incorrect public download CORS policy")
	}
	if w.Header().Get("Content-Type") != "application/octet-stream" {
		t.Error("ciphertext MIME type changed")
	}
	decoded, err := gcm.Open(nil, iv, w.Body.Bytes(), nil)
	if err != nil || !bytes.Equal(decoded, plain) {
		t.Fatal("HTTP roundtrip broke GCM authentication")
	}
	if w = call("PUT", put.RequestURI(), wire, nil); w.Code != 404 {
		t.Fatal("slot could be reused")
	}
	if err = st.DeleteFileShare(ctx, u.ID, share.ID); err != nil {
		t.Fatal(err)
	}
	if w = call("GET", get.RequestURI(), nil, nil); w.Code != 404 {
		t.Fatal("revoked download remains available")
	}
}
