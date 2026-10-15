package httpapi

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/migrate"
	"github.com/tayga/tms/internal/storage"
)

const subscriptionPrefix = "calendar.subscription."
const subscriptionLimit = 20 << 20

type calendarSource struct {
	URL      string `json:"url"`
	Kind     string `json:"kind"`
	Username string `json:"username"`
	Password string `json:"password"`
}
type calendarSubscription struct {
	ID      string           `json:"id"`
	UserID  string           `json:"user_id"`
	Name    string           `json:"display_name"`
	Secret  string           `json:"secret,omitempty"`
	Kind    string           `json:"kind"`
	Updated time.Time        `json:"updated_at"`
	Attempt time.Time        `json:"attempt_at"`
	Error   string           `json:"error,omitempty"`
	Events  []map[string]any `json:"events"`
}

// Credentials and private feed URLs are encrypted at rest, never returned by the API.
func (s *Server) subscriptionCipher() (cipher.AEAD, error) {
	key, err := migrate.ResolveKey(s.cfg.Server.SecretsKey, filepath.Dir(s.cfg.Mailstore.Root))
	if err != nil {
		return nil, err
	}
	keySum := sha256.Sum256(append(key, []byte("calendar-subscriptions-v1")...))
	block, err := aes.NewCipher(keySum[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (s *Server) subscriptionSources(ctx context.Context, user string) ([]calendarSubscription, error) {
	settings, err := s.store.ListSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := []calendarSubscription{}
	for key, value := range settings {
		if !strings.HasPrefix(key, subscriptionPrefix) || value == "" {
			continue
		}
		var sub calendarSubscription
		if json.Unmarshal([]byte(value), &sub) == nil && (user == "" || sub.UserID == user) {
			out = append(out, sub)
		}
	}
	return out, nil
}
func (s *Server) saveSubscription(ctx context.Context, sub calendarSubscription) error {
	b, err := json.Marshal(sub)
	if err != nil {
		return err
	}
	return s.store.PutSetting(ctx, subscriptionPrefix+sub.ID, string(b))
}

// Resolve and pin a public IP on every connection; redirects cannot forward secrets.
func subscriptionClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, errors.New("cannot resolve calendar host")
		}
		if len(ips) == 0 {
			return nil, errors.New("calendar host has no addresses")
		}
		for _, ip := range ips {
			if !publicCalendarIP(ip.IP) {
				return nil, errors.New("calendar host must use public addresses")
			}
		}
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
	return &http.Client{Transport: limitedCalendarTransport{transport}, Timeout: 45 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		return errors.New("use the final calendar URL; redirects are not followed")
	}}
}
func publicCalendarIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !(ip.To4() != nil && ip.To4()[0] == 100 && ip.To4()[1] >= 64 && ip.To4()[1] <= 127)
}

type limitedCalendarTransport struct{ base *http.Transport }

func (t limitedCalendarTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" {
		return nil, errors.New("HTTPS is required")
	}
	resp, err := t.base.RoundTrip(r)
	if err != nil {
		return nil, errors.New("calendar connection failed")
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, subscriptionLimit+1))
	if err != nil {
		return nil, err
	}
	if len(b) > subscriptionLimit {
		return nil, errors.New("calendar response exceeds 20 MiB")
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return resp, nil
}
func validateCalendarSource(src *calendarSource) error {
	if strings.HasPrefix(src.URL, "webcal://") {
		src.URL = "https://" + strings.TrimPrefix(src.URL, "webcal://")
	}
	u, err := url.Parse(src.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errors.New("specify an HTTPS calendar URL without embedded credentials")
	}
	if src.Kind != "ical" && src.Kind != "caldav" {
		return errors.New("kind must be ical or caldav")
	}
	return nil
}
func subscriptionEvents(id string, calendars []*ical.Calendar) ([]map[string]any, error) {
	return subscriptionEventsAt(id, calendars, time.Now().UTC())
}
func subscriptionEventsAt(id string, calendars []*ical.Calendar, now time.Time) ([]map[string]any, error) {
	overrides := map[string]bool{}
	for _, calendar := range calendars {
		for _, event := range calendar.Events() {
			if p := event.Props.Get("RECURRENCE-ID"); p != nil {
				uid, _ := event.Props.Text(ical.PropUID)
				at, err := p.DateTime(time.UTC)
				if err != nil {
					return nil, errors.New("invalid recurrence exception")
				}
				overrides[uid+"\x00"+at.UTC().Format(time.RFC3339)] = true
			}
		}
	}

	events := []map[string]any{}
	seen := map[string]bool{}
	for _, calendar := range calendars {
		for _, event := range calendar.Events() {
			uid, _ := event.Props.Text(ical.PropUID)
			if uid == "" {
				return nil, errors.New("event without UID")
			}
			recurrence := ""
			if p := event.Props.Get("RECURRENCE-ID"); p != nil {
				at, err := p.DateTime(time.UTC)
				if err != nil {
					return nil, errors.New("invalid recurrence exception")
				}
				recurrence = at.UTC().Format(time.RFC3339)
			}
			identity := uid + "\x00" + recurrence
			if seen[identity] {
				continue
			}
			seen[identity] = true
			one := ical.NewCalendar()
			one.Props = calendar.Props
			for _, child := range calendar.Children {
				if child.Name == "VTIMEZONE" {
					one.Children = append(one.Children, child)
				}
			}
			one.Children = append(one.Children, event.Component)
			var b bytes.Buffer
			if err := ical.NewEncoder(&b).Encode(one); err != nil {
				return nil, err
			}
			parsed, err := calutil.ParseICS(b.String())
			if err != nil {
				return nil, err
			}

			if strings.EqualFold(parsed.Status, "CANCELLED") {
				continue
			}
			add := func(key string, ev calutil.ParseResult) error {
				hash := sha256.Sum256([]byte(key))
				o := &storage.CalendarObject{ID: id + "-" + hex.EncodeToString(hash[:12]), CalendarID: id, UID: uid}
				events = append(events, eventToJSON(o, ev))
				if len(events) > 10000 {
					return errors.New("calendar exceeds 10000 events")
				}
				return nil
			}
			if recurrence == "" && event.Props.Get("RRULE") != nil {
				set, err := event.RecurrenceSet(time.UTC)
				if err != nil || set == nil || parsed.Start == nil {
					return nil, errors.New("unsupported recurrence or timezone")
				}
				from, to := now.AddDate(-1, 0, 0), now.AddDate(2, 0, 0)
				next := set.Iterator()
				duration := time.Duration(0)
				if parsed.End != nil {
					duration = parsed.End.Sub(*parsed.Start)
				}
				for count := 0; ; count++ {
					if count >= 100000 {
						return nil, errors.New("recurrence expansion limit exceeded")
					}
					at, ok := next()
					if !ok || at.After(to) {
						break
					}
					if at.Before(from) {
						continue
					}
					key := uid + "\x00" + at.UTC().Format(time.RFC3339)
					if overrides[key] {
						continue
					}
					instance := parsed
					instance.Start = &at
					if parsed.End != nil {
						end := at.Add(duration)
						instance.End = &end
					}
					if err := add(key, instance); err != nil {
						return nil, err
					}
				}
			} else if err := add(identity, parsed); err != nil {
				return nil, err
			}

		}
	}
	return events, nil
}
func fetchSubscriptionCalendars(ctx context.Context, src calendarSource, client *http.Client) ([]*ical.Calendar, error) {
	calendars := []*ical.Calendar{}
	if src.Kind == "ical" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
		if err != nil {
			return nil, err
		}
		if src.Username != "" {
			req.SetBasicAuth(src.Username, src.Password)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, errors.New("cannot download calendar (check URL, TLS and access)")
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("source HTTP status %d", resp.StatusCode)
		}
		calendar, err := ical.NewDecoder(resp.Body).Decode()
		if err != nil {
			return nil, errors.New("invalid iCalendar response")
		}
		calendars = append(calendars, calendar)
	} else {
		cli, err := caldav.NewClient(webdav.HTTPClientWithBasicAuth(client, src.Username, src.Password), src.URL)
		if err != nil {
			return nil, err
		}
		u, _ := url.Parse(src.URL)
		objects, err := cli.QueryCalendar(ctx, u.Path, &caldav.CalendarQuery{CompRequest: caldav.CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true}, CompFilter: caldav.CompFilter{Name: "VCALENDAR", Comps: []caldav.CompFilter{{Name: "VEVENT"}}}})
		if err != nil {
			return nil, errors.New("CalDAV query failed; specify the calendar collection URL and check credentials")
		}
		total := 0
		for _, o := range objects {
			if o.Data != nil {
				var b bytes.Buffer
				if err := ical.NewEncoder(&b).Encode(o.Data); err != nil {
					return nil, err
				}
				total += b.Len()
				if total > subscriptionLimit {
					return nil, errors.New("calendar exceeds 20 MiB")
				}
				calendars = append(calendars, o.Data)
			}
		}
	}
	return calendars, nil
}
func (s *Server) syncSubscription(ctx context.Context, sub *calendarSubscription) {
	sub.Attempt = time.Now().UTC()
	err := func() error {
		aead, err := s.subscriptionCipher()
		if err != nil {
			return err
		}
		sealed, err := base64.StdEncoding.DecodeString(sub.Secret)
		if err != nil || len(sealed) < aead.NonceSize() {
			return errors.New("invalid stored source")
		}
		plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], []byte(sub.UserID))
		if err != nil {
			return err
		}
		var src calendarSource
		if err = json.Unmarshal(plain, &src); err != nil {
			return err
		}
		if err = validateCalendarSource(&src); err != nil {
			return err
		}
		client := subscriptionClient()
		defer client.Transport.(limitedCalendarTransport).base.CloseIdleConnections()
		calendars, err := fetchSubscriptionCalendars(ctx, src, client)
		if err != nil {
			return err
		}
		events, err := subscriptionEvents(sub.ID, calendars)
		if err != nil {
			return err
		}
		sub.Events = events
		sub.Updated = time.Now().UTC()
		return nil
	}()
	sub.Error = ""
	if err != nil {
		sub.Error = err.Error()
	}
}
func (s *Server) runCalendarSubscriptions(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		s.calendarSubscriptionsMu.Lock()
		subs, err := s.subscriptionSources(ctx, "")
		if err == nil {
			for i := range subs {
				if ctx.Err() != nil {
					break
				}
				if time.Since(subs[i].Attempt) < 15*time.Minute {
					continue
				}
				s.syncSubscription(ctx, &subs[i])
				if err := s.saveSubscription(ctx, subs[i]); err != nil {
					s.log.Warn("calendar subscription save failed", "id", subs[i].ID)
				}
			}
		}
		s.calendarSubscriptionsMu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
func (s *Server) handleCalendarSubscriptions(w http.ResponseWriter, r *http.Request) {
	au, authErr := s.userFromBearer(r)
	if authErr != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if r.Method == http.MethodGet {
		subs, err := s.subscriptionSources(r.Context(), au.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot load subscriptions"})
			return
		}
		out := []map[string]any{}
		for _, sub := range subs {
			out = append(out, map[string]any{"id": sub.ID, "display_name": sub.Name, "name": sub.Name, "external": true, "writable": false, "owned": false, "color": "#3b82f6", "kind": sub.Kind, "updated_at": sub.Updated, "error": sub.Error, "events": sub.Events})
		}
		writeJSON(w, 200, map[string]any{"calendars": out})
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.calendarSubscriptionsMu.Lock()
	defer s.calendarSubscriptionsMu.Unlock()
	if r.Method == http.MethodDelete {
		id := r.URL.Query().Get("id")
		subs, err := s.subscriptionSources(r.Context(), au.ID)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		for _, sub := range subs {
			if sub.ID == id {
				if err := s.store.PutSetting(r.Context(), subscriptionPrefix+id, ""); err != nil {
					w.WriteHeader(500)
					return
				}
				w.WriteHeader(204)
				return
			}
		}
		w.WriteHeader(404)
		return
	}
	var req struct {
		calendarSource
		Name string `json:"display_name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	if err := validateCalendarSource(&req.calendarSource); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 200 {
		writeJSON(w, 400, map[string]string{"error": "calendar name required (up to 200 characters)"})
		return
	}
	subs, err := s.subscriptionSources(r.Context(), au.ID)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	if len(subs) >= 20 {
		writeJSON(w, 400, map[string]string{"error": "maximum 20 subscriptions"})
		return
	}
	aead, err := s.subscriptionCipher()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "cannot protect subscription credentials"})
		return
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		w.WriteHeader(500)
		return
	}
	plain, _ := json.Marshal(req.calendarSource)
	sub := calendarSubscription{ID: "external-" + storage.NewID(), UserID: au.ID, Name: req.Name, Kind: req.Kind, Secret: base64.StdEncoding.EncodeToString(aead.Seal(nonce, nonce, plain, []byte(au.ID)))}
	s.syncSubscription(r.Context(), &sub)
	if sub.Error != "" {
		writeJSON(w, 400, map[string]string{"error": sub.Error})
		return
	}
	if err = s.saveSubscription(r.Context(), sub); err != nil {
		w.WriteHeader(500)
		return
	}
	writeJSON(w, 201, map[string]string{"id": sub.ID})
}
