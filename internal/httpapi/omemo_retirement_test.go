package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/xmpp"
)

func TestOMEMORetirementHTTPIsolation(t *testing.T) {
	f := newFileFixture(t)
	server := xmpp.New(f.s.cfg, f.s.log, f.s.store, f.s.authn, nil)
	f.s.SetXMPP(server)
	ctx := context.Background()
	const ns = "eu.siacs.conversations.axolotl"
	const list = `<list xmlns="eu.siacs.conversations.axolotl"><device id="42"/></list>`
	for _, owner := range []string{"owner@example.com", "reader@example.com"} {
		if err := server.PublishWebOMEMO(ctx, owner, ns+".devicelist", list); err != nil {
			t.Fatal(err)
		}
		if err := server.PublishWebOMEMO(ctx, owner, ns+".bundles:42", `<bundle xmlns="eu.siacs.conversations.axolotl"><identityKey>`+owner+`</identityKey></bundle>`); err != nil {
			t.Fatal(err)
		}
	}
	path := "/api/v1/chat/omemo/devices"
	f.call(-1, "GET", path, "", 401)
	f.call(-1, "POST", path, `{"id":42}`, 401)
	f.call(0, "POST", path, `{"id":42,"identity_key":"outdated"}`, 409)
	// Even a forged owner in body and query never selects another account.
	f.call(0, "POST", path+"?owner=reader@example.com", `{"id":42,"owner":"reader@example.com","identity_key":"owner@example.com"}`, 200)
	f.call(0, "POST", path, `{"id":42,"identity_key":"owner@example.com"}`, 200)
	var result struct{ Retired []xmpp.RetiredOMEMODevice }
	json.Unmarshal(f.call(0, "GET", path, "", 200).Body.Bytes(), &result)
	if len(result.Retired) != 1 || result.Retired[0].ID != 42 {
		t.Fatal(result)
	}
	for _, body := range []string{`{"id":0}`, `{"id":2147483648}`, `{"id":-1}`, `{"id":99}`} {
		status := 400
		if strings.Contains(body, "99") {
			status = 404
		}
		f.call(0, "POST", path, body, status)
	}
	json.Unmarshal(f.call(1, "GET", path+"?owner=owner@example.com", "", 200).Body.Bytes(), &result)
	if len(result.Retired) != 0 {
		t.Fatal("leaked another user's retirements")
	}
	items, err := server.ReadWebOMEMO(ctx, "reader@example.com", ns+".bundles:42")
	if err != nil || len(items) != 1 {
		t.Fatal("changed another user's key", items, err)
	}
	if err = server.PublishWebOMEMO(ctx, "owner@example.com", ns+".bundles:42", `<bundle xmlns="eu.siacs.conversations.axolotl"/>`); err != xmpp.ErrOMEMORetired {
		t.Fatal(err)
	}
}
