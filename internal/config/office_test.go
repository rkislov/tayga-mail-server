package config

import (
	"strings"
	"testing"
)

func TestOfficeConfiguration(t *testing.T) {
	for _, tc := range []struct {
		provider, url, secret string
		valid                 bool
	}{
		{"", "", "", true},
		{"", "https://office.example.com", "", true},
		{"collabora", "https://office.example.com", "", true},
		{"onlyoffice", "https://office.example.com", strings.Repeat("x", 32), true},
		{"r7", "https://office.example.com", strings.Repeat("x", 32), true},
		{"onlyoffice", "https://office.example.com", "short", false},
		{"r7", "http://office.example.com", strings.Repeat("x", 32), false},
		{"unknown", "", "", false},
		{"onlyoffice", "https://user:pass@office.example.com", strings.Repeat("x", 32), false},
	} {
		cfg := Default()
		cfg.Server.Hostname = "mail.example.com"
		cfg.Files = FilesConfig{OfficeProvider: tc.provider, OfficeURL: tc.url, OfficeJWTSecret: tc.secret}
		if err := cfg.Validate(); (err == nil) != tc.valid {
			t.Errorf("provider=%q url=%q: %v", tc.provider, tc.url, err)
		}
	}
}
