package httpapi

import (
	"encoding/json"
	"github.com/tayga/tms/internal/fetchmail"
	"strings"
	"testing"
)

func TestMailSourcesOwnershipAndSecrets(t *testing.T) {
	f := newFileFixture(t)
	f.s.cfg.Mailstore.Root = f.s.ms.Root
	collector, e := fetchmail.New(f.s.cfg, f.s.log, f.s.store, f.s.ms)
	if e != nil {
		t.Fatal(e)
	}
	f.s.SetMailCollector(collector)
	body := `{"name":"External","protocol":"imap","host":"imap.example.com","port":993,"security":"tls","username":"external@example.com","password":"UNIQUE-TEST-SECRET","source_folder":"INBOX","target_folder":"External","interval_minutes":15,"enabled":false}`
	f.call(-1, "GET", "/api/v1/mail-sources", "", 401)
	w := f.call(0, "POST", "/api/v1/mail-sources", body, 200)
	var source fetchmail.Source
	if e = json.Unmarshal(w.Body.Bytes(), &source); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(w.Body.String(), "SECRET") || strings.Contains(w.Body.String(), "password") {
		t.Fatal("secret in response")
	}
	for _, method := range []string{"PUT", "DELETE"} {
		f.call(1, method, "/api/v1/mail-sources/"+source.ID, body, 404)
	}
	f.call(1, "POST", "/api/v1/mail-sources/"+source.ID+"/test", body, 404)
	f.call(1, "POST", "/api/v1/mail-sources/"+source.ID+"/run", "", 404)
	list := f.call(1, "GET", "/api/v1/mail-sources", "", 200)
	if strings.Contains(list.Body.String(), source.ID) {
		t.Fatal("another user's source visible")
	}
	f.call(0, "POST", "/api/v1/mail-sources/"+source.ID+"/run", "", 400)
	for i := 0; i < 4; i++ {
		f.call(0, "POST", "/api/v1/mail-sources", body, 200)
	}
	f.call(0, "POST", "/api/v1/mail-sources", body, 400)
	f.call(0, "DELETE", "/api/v1/mail-sources/"+source.ID, "", 200)
}
