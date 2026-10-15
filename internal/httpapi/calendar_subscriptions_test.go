package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestSubscriptionFeedIdentityAndChanges(t *testing.T) {
	feed := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Tayga//Test//EN\r\nBEGIN:VEVENT\r\nDTSTAMP:20261015T000000Z\r\nUID:a\r\nDTSTART:20261015T100000Z\r\nSUMMARY:First\r\nEND:VEVENT\r\nBEGIN:VEVENT\r\nDTSTAMP:20261015T000000Z\r\nUID:b\r\nDTSTART:20261015T120000Z\r\nSUMMARY:Second\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	parse := func(s string) []map[string]any {
		t.Helper()
		cal, err := ical.NewDecoder(strings.NewReader(s)).Decode()
		if err != nil {
			t.Fatal(err)
		}
		events, err := subscriptionEvents("sub", []*ical.Calendar{cal, cal})
		if err != nil {
			t.Fatal(err)
		}
		return events
	}
	events := parse(feed)
	if len(events) != 2 {
		t.Fatalf("expected two unique events: %v", events)
	}
	updated := parse(strings.Replace(feed, "First", "Changed", 1))
	if events[0]["id"] != updated[0]["id"] || updated[0]["summary"] != "Changed" {
		t.Fatal("unstable identity or stale event")
	}
	if events[0]["start"] != "2026-10-15T10:00:00Z" {
		t.Fatal(events)
	}
	empty := parse("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Tayga//Test//EN\r\nEND:VCALENDAR\r\n")
	if len(empty) != 0 {
		t.Fatal("removed source events retained")
	}
}
func TestSubscriptionDestinationRestrictions(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "::1", "192.168.1.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "fc00::1", "fe80::1", "::ffff:127.0.0.1"} {
		if publicCalendarIP(net.ParseIP(address)) {
			t.Errorf("allowed %s", address)
		}
	}
	for _, raw := range []string{"http://example.com/cal", "https://user:password@example.com/cal", "file:///etc/passwd", "https://example.com/#secret"} {
		if validateCalendarSource(&calendarSource{URL: raw, Kind: "ical"}) == nil {
			t.Errorf("allowed %s", raw)
		}
	}
	src := calendarSource{URL: "webcal://example.com/calendar.ics", Kind: "ical"}
	if err := validateCalendarSource(&src); err != nil || src.URL != "https://example.com/calendar.ics" {
		t.Fatal(src, err)
	}
}
func TestSubscriptionOwnershipAndFailedRefresh(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.Server.SecretsKey = "test-calendar-subscription-key"
	dir := t.TempDir()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	layer := auth.NewLayer(st, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("test-password")
	tenant, _ := st.CreateTenant(ctx, "t")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "example.com")
	tokens := map[string]string{}
	ids := map[string]string{}
	for _, name := range []string{"owner", "other"} {
		u, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: name + "@example.com", LocalPart: name, Enabled: true, AuthSource: "local", PasswordHash: hash})
		if err != nil {
			t.Fatal(err)
		}
		login, err := layer.LoginWithPassword(ctx, u.Email, "test-password")
		if err != nil {
			t.Fatal(err)
		}
		tokens[name] = login.Tokens.AccessToken
		ids[name] = u.ID
	}
	server := New(cfg, slog.Default(), st, layer, nil, nil, nil)
	sub := calendarSubscription{ID: "external-test", UserID: ids["owner"], Name: "Private feed", Secret: "not-a-valid-ciphertext", Updated: time.Now(), Events: []map[string]any{{"id": "existing", "summary": "Saved"}}}
	before := sub.Updated
	server.syncSubscription(ctx, &sub)
	if sub.Error == "" || len(sub.Events) != 1 || !sub.Updated.Equal(before) {
		t.Fatal("failed refresh must preserve snapshot")
	}
	if err := server.saveSubscription(ctx, sub); err != nil {
		t.Fatal(err)
	}
	call := func(user, method, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("Authorization", "Bearer "+tokens[user])
		w := httptest.NewRecorder()
		server.handleCalendarSubscriptions(w, r)
		return w
	}
	response := call("owner", "GET", "/api/v1/calendar/subscriptions")
	if response.Code != 200 || strings.Contains(response.Body.String(), sub.Secret) || strings.Contains(response.Body.String(), "user_id") {
		t.Fatal(response.Body.String())
	}
	var body struct {
		Calendars []map[string]any `json:"calendars"`
	}
	json.Unmarshal(response.Body.Bytes(), &body)
	if len(body.Calendars) != 1 {
		t.Fatal(body)
	}
	other := call("other", "GET", "/api/v1/calendar/subscriptions")
	json.Unmarshal(other.Body.Bytes(), &body)
	if len(body.Calendars) != 0 {
		t.Fatal("cross-account subscription leak")
	}
	if w := call("other", "DELETE", "/api/v1/calendar/subscriptions?id="+sub.ID); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w := call("owner", "DELETE", "/api/v1/calendar/subscriptions?id="+sub.ID); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w := call("", "GET", "/api/v1/calendar/subscriptions"); w.Code != 401 {
		t.Fatal(w.Code)
	}
}

type subscriptionRoundTrip func(*http.Request) (*http.Response, error)

func (f subscriptionRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestSubscriptionFetchICalAndCalDAV(t *testing.T) {
	feed := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Tayga//Test//EN\r\nBEGIN:VEVENT\r\nDTSTAMP:20261015T000000Z\r\nUID:remote\r\nDTSTART:20261015T100000Z\r\nSUMMARY:Remote\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	for _, kind := range []string{"ical", "caldav"} {
		t.Run(kind, func(t *testing.T) {
			client := &http.Client{Transport: subscriptionRoundTrip(func(r *http.Request) (*http.Response, error) {
				username, password, ok := r.BasicAuth()
				if !ok || username != "calendar-user" || password != "app-password" {
					t.Error("missing source credentials")
				}
				status := 200
				body := feed
				if kind == "caldav" {
					if r.Method != "REPORT" || r.URL.Path != "/calendars/work/" || r.Header.Get("Depth") != "1" {
						t.Errorf("invalid DAV request: %s %s", r.Method, r.URL.Path)
					}
					request, _ := io.ReadAll(r.Body)
					if !strings.Contains(string(request), "VEVENT") {
						t.Error("missing event filter")
					}
					status = 207
					var escaped bytes.Buffer
					xml.EscapeText(&escaped, []byte(feed))
					body = `<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:response><d:href>/calendars/work/event.ics</d:href><d:propstat><d:prop><d:getetag>"1"</d:getetag><c:calendar-data>` + escaped.String() + `</c:calendar-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`
				} else if r.Method != "GET" {
					t.Error("iCalendar must use GET")
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/xml"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})}
			calendars, err := fetchSubscriptionCalendars(context.Background(), calendarSource{URL: "https://calendar.example.com/calendars/work/", Kind: kind, Username: "calendar-user", Password: "app-password"}, client)
			if err != nil {
				t.Fatal(err)
			}
			events, err := subscriptionEvents("external-test", calendars)
			if err != nil || len(events) != 1 || events[0]["summary"] != "Remote" {
				t.Fatalf("%v %v", events, err)
			}
		})
	}
}

func TestSubscriptionRecurrenceException(t *testing.T) {
	feed := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Tayga//Test//EN
BEGIN:VEVENT
DTSTAMP:20261015T000000Z
UID:daily
DTSTART:20261015T100000Z
DTEND:20261015T110000Z
RRULE:FREQ=DAILY;COUNT=3
EXDATE:20261016T100000Z
SUMMARY:Daily
END:VEVENT
BEGIN:VEVENT
DTSTAMP:20261015T000000Z
UID:daily
RECURRENCE-ID:20261017T100000Z
DTSTART:20261017T120000Z
DTEND:20261017T130000Z
SUMMARY:Moved
END:VEVENT
END:VCALENDAR
`
	cal, err := ical.NewDecoder(strings.NewReader(feed)).Decode()
	if err != nil {
		t.Fatal(err)
	}
	events, err := subscriptionEventsAt("sub", []*ical.Calendar{cal}, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1]["summary"] != "Moved" || events[1]["start"] != "2026-10-17T12:00:00Z" {
		t.Fatalf("wrong recurrence: %v", events)
	}
}
