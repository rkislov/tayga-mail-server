package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestWelcomeVersionsPublishRestoreInheritanceAndPermissions(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	u, err := f.s.store.GetUserByID(ctx, f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.store.UpdateUserRoles(ctx, u.ID, "tenant_admin"); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/admin/welcome/tenants/" + u.TenantID
	domainPath := "/api/v1/admin/welcome/domains/" + u.DomainID
	f.call(-1, "GET", path, "", 401)
	f.call(1, "GET", path, "", 403)
	f.call(1, "POST", domainPath, `{"action":"save"}`, 403)
	f.call(0, "PUT", path, `{}`, 405)
	f.call(0, "GET", "/api/v1/admin/welcome/unknown/anything", "", 404)
	f.call(0, "POST", path, `{"action":"save","extra":true}`, 400)
	f.call(0, "POST", path, `{"action":"save"} {}`, 400)
	f.call(0, "POST", path, `{"action":"mode","mode":"own","expected":0}`, 400)
	post := func(endpoint string, c storage.WelcomeChange, code int) *storage.WelcomeTemplate {
		t.Helper()
		raw, _ := json.Marshal(c)
		w := f.call(0, "POST", endpoint, string(raw), code)
		if code != 200 {
			return nil
		}
		var result storage.WelcomeTemplate
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return &result
	}
	get := func(endpoint string) (storage.WelcomeTemplate, *storage.WelcomeRevision) {
		t.Helper()
		var result struct {
			Template  storage.WelcomeTemplate
			Effective *storage.WelcomeRevision
		}
		w := f.call(0, "GET", endpoint, "", 200)
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Template, result.Effective
	}
	draft := storage.WelcomeChange{Action: "save", Content: storage.WelcomeRevision{Subject: "Welcome {{name}}", HTML: `<h1>Hello {{name}}</h1><p>Use <a href="https://example.com">webmail</a>.</p><script>alert(1)</script>`, Comment: "First"}}
	v := post(path, draft, 200)
	if v.Generation != 1 || len(v.Revisions) != 1 || v.Published != 0 || v.Mode != "off" || v.Revisions[0].Author != u.Email || v.Revisions[0].Text == "" || strings.Contains(v.Revisions[0].HTML, "script") {
		t.Fatalf("saved: %+v", v)
	}
	if _, effective := get(domainPath); effective != nil {
		t.Fatal("draft affected inherited template")
	}
	post(path, storage.WelcomeChange{Action: "publish", Expected: 1, Revision: 1}, 200)
	if _, effective := get(domainPath); effective == nil || effective.Number != 1 || effective.Author != "" || effective.Comment != "" {
		t.Fatalf("inheritance: %+v", effective)
	}
	draft.Expected = 2
	draft.Content.Subject = "Second"
	draft.Content.HTML = "<p>Second body</p>"
	post(path, draft, 200)
	if _, effective := get(domainPath); effective == nil || effective.Number != 1 {
		t.Fatal("new draft replaced publication")
	}
	post(path, storage.WelcomeChange{Action: "publish", Expected: 3, Revision: 2}, 200)
	post(path, storage.WelcomeChange{Action: "publish", Expected: 3, Revision: 1}, 409)
	post(path, storage.WelcomeChange{Action: "restore", Expected: 4, Revision: 99}, 404)
	restored := post(path, storage.WelcomeChange{Action: "restore", Expected: 4, Revision: 1, Content: storage.WelcomeRevision{Comment: "Rollback"}}, 200)
	if restored.Published != 2 || len(restored.Revisions) != 3 || restored.Revisions[2].RestoredFrom != 1 || restored.Revisions[2].HTML != restored.Revisions[0].HTML || restored.Revisions[2].Comment != "Rollback" {
		t.Fatalf("restored: %+v", restored)
	}
	post(path, storage.WelcomeChange{Action: "publish", Expected: 5, Revision: 3}, 200)
	draft.Expected = 0
	draft.Content.Subject = "Domain draft"
	post(domainPath, draft, 200)
	if _, effective := get(domainPath); effective == nil || effective.Number != 3 {
		t.Fatal("domain draft broke inheritance")
	}
	post(domainPath, storage.WelcomeChange{Action: "publish", Expected: 1, Revision: 1}, 200)
	if _, effective := get(domainPath); effective == nil || effective.Subject != "Domain draft" {
		t.Fatal("domain override")
	}
	post(domainPath, storage.WelcomeChange{Action: "mode", Expected: 2, Mode: "off"}, 200)
	if _, effective := get(domainPath); effective != nil {
		t.Fatal("domain off fell back to tenant")
	}
	post(domainPath, storage.WelcomeChange{Action: "mode", Expected: 3, Mode: "inherit"}, 200)
	if _, effective := get(domainPath); effective == nil || effective.Number != 3 {
		t.Fatal("inherit not restored")
	}
	before, _ := get(path)
	f.call(0, "POST", path, `{"action":"preview","content":{"subject":"Preview","html":"<p>Hello</p><script>bad()</script>"}}`, 200)
	after, _ := get(path)
	if after.Generation != before.Generation || len(after.Events) != len(before.Events) {
		t.Fatal("preview mutated history")
	}
	if len(after.Events) != 6 {
		t.Fatal("audit", after.Events)
	}
	other, err := f.s.store.CreateTenant(ctx, "other-welcome")
	if err != nil {
		t.Fatal(err)
	}
	od, err := f.s.store.CreateDomain(ctx, other.ID, "other-welcome.example")
	if err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"/api/v1/admin/welcome/tenants/" + other.ID, "/api/v1/admin/welcome/domains/" + od.ID} {
		f.call(0, "GET", endpoint, "", 403)
		f.call(0, "POST", endpoint, `{"action":"restore","revision":1}`, 403)
	}
	// A domain admin can read the inherited published letter but not tenant drafts.
	if err = f.s.store.UpdateUserRoles(ctx, u.ID, "domain_admin"); err != nil {
		t.Fatal(err)
	}
	if err = f.s.store.SetDomainAdminDomains(ctx, u.ID, []string{u.DomainID}); err != nil {
		t.Fatal(err)
	}
	f.call(0, "GET", path, "", 403)
	f.call(0, "GET", domainPath, "", 200)
	if err = f.s.store.UpdateUserRoles(ctx, u.ID, "global_admin"); err != nil {
		t.Fatal(err)
	}
	f.call(0, "GET", "/api/v1/admin/welcome/tenants/"+other.ID, "", 200)
}

func TestWelcomeHTMLSanitizationAndValidation(t *testing.T) {
	input := `<html><head><style>bad</style></head><body onload="bad()"><h1>Welcome</h1><p style="background:url(javascript:bad)"><a href="javascript:alert(1)" onclick="bad()">bad link</a><img src="data:image/svg+xml,bad" onerror="bad()"><img src="https://example.com/logo.png" alt="logo"><iframe src="https://evil.example">bad</iframe><svg><script>bad</script></svg><table><tr><td colspan="2">Cell</td></tr></table><!-- hidden --></p></body></html>`
	cleaned, err := sanitizeWelcomeHTML(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"javascript", "onclick", "onload", "onerror", "<script", "<style", "<svg", "<iframe", "data:", "<!--", "style="} {
		if strings.Contains(cleaned, bad) {
			t.Fatalf("unsafe %s in %s", bad, cleaned)
		}
	}
	for _, want := range []string{"<h1>Welcome</h1>", `src="https://example.com/logo.png"`, `colspan="2"`} {
		if !strings.Contains(cleaned, want) {
			t.Fatalf("lost %s in %s", want, cleaned)
		}
	}
	cases := []storage.WelcomeRevision{
		{Subject: "", HTML: "<p>x</p>"},
		{Subject: "injected\r\nBCC: x", HTML: "<p>x</p>"},
		{Subject: "ok", HTML: "<script>bad</script>"},
		{Subject: "{{password}}", HTML: "<p>x</p>"},
		{Subject: "ok", HTML: "<p>{{unknown}}</p>"},
		{Subject: "ok", HTML: "<p>{{name</p>"},
		{Subject: "ok", HTML: strings.Repeat("x", (256<<10)+1)},
	}
	for _, c := range cases {
		if normalizeWelcomeContent(&c) == nil {
			t.Fatal("accepted invalid content", c.Subject)
		}
	}
	c := storage.WelcomeRevision{Subject: "Hi {{name}}", HTML: `<p>{{email}} @ {{domain}} · {{tenant}}</p>`}
	if err = normalizeWelcomeContent(&c); err != nil {
		t.Fatal(err)
	}
	// Repeated sanitization is stable, including malformed HTML from source mode.
	for _, raw := range []string{input, `<a href="&#106;avascript:alert(1)">x</a>`, `<math><mtext><table><mglyph><style><!--</style><img title="--><img src=1 onerror=alert(1)>">`} {
		a, _ := sanitizeWelcomeHTML(raw)
		b, _ := sanitizeWelcomeHTML(a)
		if a != b {
			t.Fatalf("unstable sanitization %q -> %q", a, b)
		}
	}
}

func TestWelcomeConcurrentSaveAndCascade(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	u, err := f.s.store.GetUserByID(ctx, f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	d, err := f.s.store.CreateDomain(ctx, u.TenantID, "welcome-cas.example")
	if err != nil {
		t.Fatal(err)
	}
	change := storage.WelcomeChange{Action: "save", Content: storage.WelcomeRevision{Subject: "hello", HTML: "<p>hello</p>"}}
	for _, expected := range []int64{0, 1} {
		change.Expected = expected
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, e := f.s.store.ChangeWelcomeTemplate(ctx, "domains", d.ID, u.Email, change)
				results <- e
			}()
		}
		wg.Wait()
		close(results)
		success, conflict := 0, 0
		for e := range results {
			if e == nil {
				success++
			} else if errors.Is(e, storage.ErrWelcomeConflict) {
				conflict++
			} else {
				t.Fatal(e)
			}
		}
		if success != 1 || conflict != 1 {
			t.Fatal("CAS", success, conflict)
		}
	}
	if err = f.s.store.DeleteDomain(ctx, u.TenantID, d.ID); err != nil {
		t.Fatal(err)
	}
	all, err := f.s.store.ListWelcomeTemplates(ctx, u.TenantID)
	if err != nil || len(all) != 0 {
		t.Fatal("orphan template", all, err)
	}
}
