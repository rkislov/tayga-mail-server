package spam

import (
	"context"
	"net"
	"testing"
)

func TestDNSBLScoringAndAuthenticatedBypass(t *testing.T) {
	calls := 0
	checker := &DNSBL{next: Noop{}, zones: []string{"listed.test", "error.test"}, score: 3, lookup: func(ctx context.Context, host string) ([]net.IP, error) {
		calls++
		if host == "8.8.8.8.listed.test" {
			return []net.IP{net.ParseIP("127.0.0.2")}, nil
		}
		return []net.IP{net.ParseIP("127.255.255.254")}, nil
	}}
	result, err := checker.Check(context.Background(), Meta{IP: "8.8.8.8"}, nil)
	if err != nil || result.Score != 3 || len(result.Symbols) != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	calls = 0
	result, err = checker.Check(context.Background(), Meta{IP: "8.8.8.8", User: "sender@example.com"}, nil)
	if err != nil || calls != 0 || result.Score != 0 {
		t.Fatal("authenticated peer checked")
	}
}
