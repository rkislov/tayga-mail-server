package migrate

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Each job owns its transport. Certificate exceptions never alter the global
// transport and credentials cannot follow redirects to a different origin.
func davHTTPClient(rawURL string, skipVerify bool) (*http.Client, error) {
	origin, err := url.Parse(rawURL)
	if err != nil || origin.Host == "" || (origin.Scheme != "https" && origin.Scheme != "http") {
		return nil, fmt.Errorf("valid HTTP(S) URL required")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: skipVerify} // explicit per-job user setting
	return &http.Client{Transport: transport, Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.Scheme != origin.Scheme || !strings.EqualFold(req.URL.Host, origin.Host) {
			return fmt.Errorf("DAV redirect to another origin is not allowed")
		}
		return nil
	}}, nil
}
