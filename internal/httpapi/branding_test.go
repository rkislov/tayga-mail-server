package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestBrandingOwnershipValidationAndReset(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	u, err := f.s.store.GetUserByID(ctx, f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.store.UpdateUserRoles(ctx, u.ID, "tenant_admin"); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/admin/branding/tenants/" + u.TenantID
	domainPath := "/api/v1/admin/branding/domains/" + u.DomainID
	const resolved = "/api/v1/me/branding"
	f.call(1, "PUT", path, `{}`, 403)
	f.call(1, "PUT", domainPath, `{}`, 403)
	f.call(1, "PUT", resolved, `{}`, 405)
	f.call(-1, "GET", path, "", 401)
	f.call(-1, "PUT", path, `{}`, 401)
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 20, 10))); err != nil {
		t.Fatal(err)
	}
	logo := "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
	raw, _ := json.Marshal(brandingConfig{Enabled: true, Name: "  Компания  ", Image: logo})
	f.call(0, "PUT", path, string(raw), 200)
	var saved brandingConfig
	if err := json.Unmarshal(f.call(0, "GET", path, "", 200).Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Name != "Компания" || saved.Image != logo {
		t.Fatalf("branding not persisted: %+v", saved)
	}
	// The tenant branding applies to all its users, until a domain overrides it.
	f.call(1, "GET", path, "", 403)
	if got := f.call(1, "GET", resolved, "", 200).Body.String(); !strings.Contains(got, "Компания") {
		t.Fatal("tenant branding not inherited")
	}
	f.call(0, "PUT", domainPath, `{"enabled":true,"name":"Домен","image":""}`, 200)
	if got := f.call(1, "GET", resolved, "", 200).Body.String(); !strings.Contains(got, "Домен") || strings.Contains(got, logo) {
		t.Fatal("domain override not applied")
	}
	f.call(0, "PUT", domainPath, `{"enabled":false}`, 200)
	if got := f.call(1, "GET", resolved, "", 200).Body.String(); !strings.Contains(got, "Компания") {
		t.Fatal("domain reset did not restore tenant branding")
	}
	other, err := f.s.store.CreateTenant(ctx, "other")
	if err != nil {
		t.Fatal(err)
	}
	otherDomain, err := f.s.store.CreateDomain(ctx, other.ID, "other.example")
	if err != nil {
		t.Fatal(err)
	}
	f.call(0, "PUT", "/api/v1/admin/branding/tenants/"+other.ID, `{}`, 403)
	f.call(0, "PUT", "/api/v1/admin/branding/domains/"+otherDomain.ID, `{}`, 403)
	if err := f.s.store.UpdateUserRoles(ctx, u.ID, "domain_admin"); err != nil {
		t.Fatal(err)
	}
	f.call(0, "PUT", path, `{}`, 403)
	if err := f.s.store.SetDomainAdminDomains(ctx, u.ID, []string{u.DomainID}); err != nil {
		t.Fatal(err)
	}
	f.call(0, "PUT", domainPath, `{"enabled":false}`, 200)
	if err := f.s.store.UpdateUserRoles(ctx, u.ID, "global_admin"); err != nil {
		t.Fatal(err)
	}
	f.call(0, "PUT", "/api/v1/admin/branding/tenants/"+other.ID, `{"enabled":true,"name":"Другой"}`, 200)
	if got := f.call(1, "GET", resolved, "", 200).Body.String(); strings.Contains(got, "Другой") {
		t.Fatal("foreign tenant branding leaked")
	}

	for _, payload := range []string{
		`{"name":"x","hide_attribution":true}`,
		`{"image":"https://example.com/logo.png"}`,
		`{"image":"data:image/svg+xml;base64,PHN2Zz4="}`,
		`{"image":"data:image/png;base64,bm90IGEgcG5n"}`,
		`{"name":"` + strings.Repeat("я", 61) + `"}`,
		`{"name":"bad\nname"}`,
		`{} {}`,
	} {
		f.call(0, "PUT", path, payload, 400)
	}
	if got := f.call(0, "GET", path, "", 200).Body.String(); !strings.Contains(got, "Компания") {
		t.Fatal("invalid write changed saved branding")
	}
	f.call(0, "POST", path, `{}`, 405)
	f.call(0, "PUT", path, `{"name":"","image":""}`, 200)
	if err := json.Unmarshal(f.call(0, "GET", path, "", 200).Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Name != "" || saved.Image != "" {
		t.Fatal("reset failed")
	}
	// Reject dimension bombs before decoding pixels, and truncated image contents.
	b.Reset()
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4097, 1))); err != nil {
		t.Fatal(err)
	}
	if validBrandingImage("data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())) {
		t.Fatal("oversized dimensions accepted")
	}
	if validBrandingImage("data:image/png;base64," + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0}, (1<<20)+1))) {
		t.Fatal("oversized file accepted")
	}
	if validBrandingImage(strings.Replace(logo, "image/png", "image/jpeg", 1)) {
		t.Fatal("wrong MIME accepted")
	}
}
