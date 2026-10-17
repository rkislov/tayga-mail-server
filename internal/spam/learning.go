package spam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func LearningEnabled(c config.SpamConfig) bool {
	return c.Enabled && !c.Learning.Disabled && (c.Backend == "rspamd" || c.Backend == "")
}
func Controller(c config.SpamConfig) (*Rspamd, error) {
	address := c.Learning.URL
	if address == "" {
		address = c.URL
		if address == "" {
			address = "http://127.0.0.1:11333"
		}
	}
	u, err := url.Parse(address)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("invalid Rspamd controller URL")
	}
	if c.Learning.URL == "" && u.Port() == "11333" {
		u.Host = net.JoinHostPort(u.Hostname(), "11334")
	}
	password := c.Learning.Password
	if password == "" {
		password = c.Password
	}
	r := NewRspamd(u.String(), password, 10*time.Second)
	r.client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	return r, nil
}
func (r *Rspamd) Learn(ctx context.Context, user, class string, data []byte) (string, error) {
	if class != "spam" && class != "ham" {
		return "", fmt.Errorf("invalid learning class")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.base+"/learn"+class, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("invalid learning request")
	}
	req.Header.Set("Content-Type", "message/rfc822")
	req.Header.Set("User", user)
	req.Header.Set("Deliver-To", user)
	req.Header.Set("Password", r.password)
	res, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("controller connection failed")
	}
	defer res.Body.Close()
	var response struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(&response); err != nil {
		return "", fmt.Errorf("controller HTTP %d: invalid response", res.StatusCode)
	}
	if response.Success && res.StatusCode >= 200 && res.StatusCode < 300 {
		return class, nil
	}
	lower := strings.ToLower(response.Error)
	if strings.Contains(lower, "already learned") || strings.Contains(lower, "already learnt") {
		return "duplicate", nil
	}
	// Do not log controller bodies: they may contain message content or credentials.
	return "", fmt.Errorf("controller HTTP %d: learning rejected", res.StatusCode)
}
func (r *Rspamd) Statistics(ctx context.Context) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.base+"/stat", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Password", r.password)
	res, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("controller unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("controller HTTP %d", res.StatusCode)
	}
	var raw map[string]any
	if err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid controller statistics")
	}
	result := map[string]any{}
	for _, key := range []string{"version", "scanned", "learned", "spam_count", "ham_count", "statfiles"} {
		if value, ok := raw[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

type LearningWorker struct {
	Store     *storage.Store
	Mailstore *mailstore.Store
	Config    func() config.SpamConfig
	Log       *slog.Logger
}

func (w *LearningWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	lastCleanup := time.Time{}
	for {
		if time.Since(lastCleanup) > time.Hour {
			w.Store.CleanupProtection(ctx)
			lastCleanup = time.Now()
		}
		w.Process(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (w *LearningWorker) Process(ctx context.Context) {
	cfg := w.Config()
	if !LearningEnabled(cfg) {
		return
	}
	client, err := Controller(cfg)
	if err != nil {
		w.Log.Warn("rspamd learning configuration invalid")
		return
	}
	jobs, err := w.Store.DueSpamLearning(ctx)
	if err != nil {
		w.Log.Warn("rspamd learning queue unavailable")
		return
	}
	for _, job := range jobs {
		if ctx.Err() != nil {
			return
		}
		claimed, claimErr := w.Store.ClaimSpamLearning(ctx, job)
		if claimErr != nil || !claimed {
			continue
		}
		outcome := ""
		detail := ""
		msg, e := w.Store.GetMessageByID(ctx, job.MessageID)
		user, ue := w.Store.GetUserByID(ctx, job.UserID)
		if e != nil || ue != nil {
			continue
		}
		if !user.Enabled || msg.Size > 64<<20 {
			outcome = "skipped"
			detail = "disabled account or message exceeds 64 MiB"
		} else {
			raw, re := w.Mailstore.Read(msg.FilePath)
			if re != nil {
				e = fmt.Errorf("message content unavailable")
			} else {
				outcome, e = client.Learn(ctx, user.Email, job.Class, raw)
			}
			if e != nil {
				outcome = "error"
				detail = e.Error()
			}
		}
		if ctx.Err() != nil {
			return
		}
		if e = w.Store.FinishSpamLearning(ctx, job, outcome, detail); e != nil {
			w.Log.Warn("rspamd learning result not saved", "message_id", job.MessageID)
			continue
		}
		w.Log.Info("rspamd learning", "message_id", job.MessageID, "classification", job.Class, "result", outcome, "attempt", job.Attempts+1, "detail", detail)
	}
}
