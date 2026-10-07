package smtp

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseSTSPolicy(t *testing.T) {
	body := "version: STSv1\nmode: enforce\nmx: mail.example.com\nmx: *.example.com\nmax_age: 86400\n"
	pol, err := parseSTSPolicy(body)
	if err != nil {
		t.Fatal(err)
	}
	if pol.Mode != "enforce" || pol.MaxAge != 24*time.Hour || len(pol.MXs) != 2 {
		t.Fatalf("%+v", pol)
	}
}

func TestMatchSTSMX(t *testing.T) {
	patterns := []string{"mail.example.com", "*.example.com"}
	if !matchSTSMX(patterns, "mail.example.com") {
		t.Fatal("exact")
	}
	if !matchSTSMX(patterns, "mx1.example.com") {
		t.Fatal("wildcard")
	}
	if matchSTSMX(patterns, "example.com") {
		t.Fatal("bare domain must not match *.")
	}
	if matchSTSMX(patterns, "a.b.example.com") {
		t.Fatal("multi-label must not match single *")
	}
}

func TestSTSResolverPolicy(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/mta-sts.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("version: STSv1\nmode: testing\nmx: mx.test\nmax_age: 60\n"))
	}))
	defer srv.Close()

	r := newSTSResolver(5*time.Second, false, nil)
	r.lookupTXT = func(name string) ([]string, error) {
		if name != "_mta-sts.example.com" {
			t.Fatalf("txt %s", name)
		}
		return []string{"v=STSv1; id=abc"}, nil
	}
	// Route https://mta-sts.example.com/... to the test server.
	r.client = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("tcp", srv.Listener.Addr().String())
			},
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	pol, err := r.Policy(context.Background(), "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if pol == nil || pol.Mode != "testing" || pol.ID != "abc" || len(pol.MXs) != 1 {
		t.Fatalf("%+v", pol)
	}
}

func TestFilterMXBySTS(t *testing.T) {
	pol := &stsPolicy{Mode: "enforce", MXs: []string{"*.example.com"}}
	got := filterMXBySTS([]string{"a.example.com", "other.test"}, pol)
	if len(got) != 1 || got[0] != "a.example.com" {
		t.Fatalf("%v", got)
	}
}

func TestBuildTLSReportJSON(t *testing.T) {
	begin := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := begin.Add(24 * time.Hour)
	body, err := buildTLSReportJSON("Tayga", "t@test", "mail.test", "example.com", begin, end, []*tlsEvent{
		{PolicyDomain: "example.com", MXHost: "mx.example.com", ResultType: "successful-session", Count: 2},
		{PolicyDomain: "example.com", MXHost: "mx.example.com", ResultType: "starttls-failure", Count: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, `"total-successful-session-count": 2`) {
		t.Fatal(s)
	}
	if !strings.Contains(s, "starttls-failure") {
		t.Fatal(s)
	}
}

func TestTLSReporterLookupRUA(t *testing.T) {
	r := newTLSReporter("T", "c@t", "mail.test", time.Hour, nil)
	r.lookupTXT = func(name string) ([]string, error) {
		if name != "_smtp._tls.example.com" {
			t.Fatalf("%s", name)
		}
		return []string{"v=TLSRPTv1; rua=mailto:tls@example.com,mailto:TLS@example.com"}, nil
	}
	got, err := r.lookupRUA("example.com")
	if err != nil || len(got) != 1 || got[0] != "tls@example.com" {
		t.Fatalf("%v %v", got, err)
	}
}
