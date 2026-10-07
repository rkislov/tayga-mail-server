package spam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Rspamd calls the Rspamd HTTP /checkv2 API.
type Rspamd struct {
	base     string
	password string
	client   *http.Client
}

func NewRspamd(baseURL, password string, timeout time.Duration) *Rspamd {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &Rspamd{
		base:     baseURL,
		password: password,
		client:   &http.Client{Timeout: timeout},
	}
}

func (r *Rspamd) Name() string { return "rspamd" }

func (r *Rspamd) Check(ctx context.Context, meta Meta, data []byte) (*Result, error) {
	u, err := url.Parse(r.base + "/checkv2")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "message/rfc822")
	if meta.From != "" {
		req.Header.Set("From", meta.From)
	}
	if meta.To != "" {
		req.Header.Set("Rcpt", meta.To)
	}
	if meta.Helo != "" {
		req.Header.Set("Helo", meta.Helo)
	}
	if meta.IP != "" {
		req.Header.Set("IP", meta.IP)
	}
	if meta.User != "" {
		req.Header.Set("User", meta.User)
	}
	if meta.Hostname != "" {
		req.Header.Set("Hostname", meta.Hostname)
	}
	if r.password != "" {
		req.Header.Set("Password", r.password)
	}

	res, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rspamd: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("rspamd: HTTP %s: %s", res.Status, bytes.TrimSpace(body))
	}

	var raw struct {
		Score         float64                    `json:"score"`
		RequiredScore float64                    `json:"required_score"`
		Action        string                     `json:"action"`
		Symbols       map[string]json.RawMessage `json:"symbols"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("rspamd decode: %w", err)
	}
	out := &Result{
		Score:    raw.Score,
		Required: raw.RequiredScore,
		Action:   strings.ToLower(strings.TrimSpace(raw.Action)),
		Scanner:  "rspamd",
	}
	for name := range raw.Symbols {
		out.Symbols = append(out.Symbols, name)
	}
	return out, nil
}
