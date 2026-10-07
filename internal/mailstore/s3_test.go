package mailstore

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestS3BlobAgainstFakeServer(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "unsigned", http.StatusForbidden)
			return
		}
		// path-style: /bucket/key…
		path := strings.TrimPrefix(r.URL.Path, "/tayga-mail/")
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			objects[path] = body
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			data, ok := objects[path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(data)
		case http.MethodDelete:
			delete(objects, path)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	blob, err := NewS3Blob(S3Config{
		Endpoint:  srv.URL,
		Region:    "us-east-1",
		Bucket:    "tayga-mail",
		AccessKey: "ak",
		SecretKey: "sk",
		Prefix:    "maildir/",
		PathStyle: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := blob.Put(ctx, "u@ex.com/new/msg", []byte("body")); err != nil {
		t.Fatal(err)
	}
	got, err := blob.Get(ctx, "u@ex.com/new/msg")
	if err != nil || string(got) != "body" {
		t.Fatalf("get: %q %v", got, err)
	}
	if err := blob.Delete(ctx, "u@ex.com/new/msg"); err != nil {
		t.Fatal(err)
	}
	if _, err := blob.Get(ctx, "u@ex.com/new/msg"); err == nil {
		t.Fatal("expected miss")
	}
}
