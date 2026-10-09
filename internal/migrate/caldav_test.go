package migrate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestCalDAVSOGoCollectionExplicitEventFilter(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tenant, _ := st.CreateTenant(ctx, "t")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "ex.com")
	user, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "u@ex.com", LocalPart: "u", Enabled: true, AuthSource: "local"})
	if err != nil {
		t.Fatal(err)
	}
	eventReports := 0
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(207)
		if r.Method == "PROPFIND" {
			fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:response><d:href>/calendar/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/><c:calendar/></d:resourcetype><d:displayname>Source</d:displayname></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
			return
		}
		if strings.Contains(string(body), `name="VEVENT"`) {
			eventReports++
			fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:response><d:href>/calendar/one.ics</d:href><d:propstat><d:prop><d:getetag>"one"</d:getetag><c:calendar-data><![CDATA[BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:one
DTSTAMP:20261009T060000Z
DTSTART:20261009T070000Z
DTEND:20261009T080000Z
SUMMARY:Imported
END:VEVENT
END:VCALENDAR
]]></c:calendar-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
			return
		}
		fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:"/>`)
	}))
	defer source.Close()
	cfg := config.Default()
	cfg.Server.SecretsKey = "calendar test key"
	svc, err := New(cfg, nil, st, mailstore.New(filepath.Join(dir, "mail")))
	if err != nil {
		t.Fatal(err)
	}
	cipher, _ := encryptPassword(svc.key, "secret")
	job := &Job{ID: "test", UserID: user.ID, URL: source.URL + "/calendar/", Username: "source", PasswordCiphertext: cipher}
	if err = svc.runCalDAV(ctx, job); err != nil {
		t.Fatal(err)
	}
	if eventReports != 1 || job.Copied != 1 || job.Errors != 0 {
		t.Fatalf("reports=%d job=%+v", eventReports, job)
	}
	cal, _ := st.GetCalendarByName(ctx, user.ID, "default")
	objects, _ := st.ListCalendarObjects(ctx, cal.ID)
	if len(objects) != 1 || !strings.Contains(objects[0].Data, "SUMMARY:Imported") {
		t.Fatal("event missing")
	}
}
