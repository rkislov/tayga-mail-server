package flowsync

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggingExcludesSecrets(t *testing.T) {
	var out bytes.Buffer
	h := logRequests(slog.New(slog.NewJSONHandler(&out, nil)), "ews", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceRequest(r).Command = "GetFolder"
		traceRequest(r).Result = "unsupported_operation"
		w.WriteHeader(401)
		_, _ = w.Write([]byte("private response"))
	}))
	r := httptest.NewRequest("POST", "/EWS/Exchange.asmx?password=query-secret", strings.NewReader("private body"))
	r.Header.Set("Authorization", "Bearer token-secret")
	r.Header.Set("Cookie", "session=cookie-secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	for _, secret := range []string{"query-secret", "token-secret", "cookie-secret", "private body", "private response"} {
		if strings.Contains(out.String(), secret) {
			t.Fatalf("logged secret %s", secret)
		}
	}
	for _, want := range []string{`"status":401`, `"command":"GetFolder"`, `"result":"unsupported_operation"`, `"level":"WARN"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %s: %s", want, out.String())
		}
	}
	if w.Code != 401 || w.Body.String() != "private response" {
		t.Fatal("middleware changed response")
	}
}
