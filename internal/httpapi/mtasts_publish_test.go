package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
)

func TestMTASTSPublish(t *testing.T) {
	s := &Server{cfg: &config.Config{
		Server: config.ServerConfig{Hostname: "mail.example.com"},
		SMTP: config.SMTPConfig{
			MTASTS: config.MTASTSConfig{
				Publish: config.MTASTSPublishConfig{
					Enabled: true,
					Mode:    "enforce",
					MaxAge:  3600,
					MX:      []string{"mail.example.com", "*.example.com"},
				},
			},
		},
	}}
	req := httptest.NewRequest(http.MethodGet, "/.well-known/mta-sts.txt", nil)
	rec := httptest.NewRecorder()
	s.handleMTASTSPolicy(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"version: STSv1",
		"mode: enforce",
		"mx: mail.example.com",
		"mx: *.example.com",
		"max_age: 3600",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Fatalf("ct %s", ct)
	}
}

func TestMTASTSPublishDisabled(t *testing.T) {
	s := &Server{cfg: &config.Config{}}
	req := httptest.NewRequest(http.MethodGet, "/.well-known/mta-sts.txt", nil)
	rec := httptest.NewRecorder()
	s.handleMTASTSPolicy(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code %d", rec.Code)
	}
}
