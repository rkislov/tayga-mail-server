package xmpp

import (
	"context"
	"encoding/xml"
	"strings"
	"testing"
)

func TestWebOMEMOCiphertextAndPublication(t *testing.T) {
	srv, _, u := testServer(t)
	ctx := context.Background()
	raw := `<encrypted xmlns="eu.siacs.conversations.axolotl"><header sid="42"><key rid="7" prekey="true">YWJj</key><iv>AAAAAAAAAAAAAAAA</iv></header><payload>YWJj</payload><body>not allowed</body></encrypted>`
	out, err := validateWebCiphertext(raw)
	if err != nil || strings.Contains(out, "not allowed") {
		t.Fatal(out, err)
	}
	ev, err := srv.SendWebOMEMO(ctx, u.Email, u.Email, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !ev.Encrypted || ev.Body != "" || ev.OMEMO == "" {
		t.Fatal(ev)
	}
	for _, bad := range []string{strings.Replace(raw, `sid="42"`, `sid="0"`, 1), strings.Replace(raw, `rid="7"`, `rid="0"`, 1), strings.Replace(raw, "AAAAAAAAAAAAAAAA", "!", 1)} {
		if _, err := validateWebCiphertext(bad); err == nil {
			t.Fatal("invalid accepted")
		}
	}
	node := legacyOMEMO + ".devicelist"
	if err := srv.PublishWebOMEMO(ctx, u.Email, node, `<list xmlns="eu.siacs.conversations.axolotl"><device id="42"/></list>`); err != nil {
		t.Fatal(err)
	}
	items, err := srv.ReadWebOMEMO(ctx, u.Email, node)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	if err := srv.PublishWebOMEMO(ctx, u.Email, "other", "<list/>"); err == nil {
		t.Fatal("other node accepted")
	}
}
func TestWebOMEMOInheritedPrefixes(t *testing.T) {
	raw := `<message xmlns:o="eu.siacs.conversations.axolotl"><o:encrypted><o:header sid="42"><o:key rid="7">YWJj</o:key><o:iv>AAAAAAAAAAAAAAAA</o:iv></o:header><o:payload>YWJj</o:payload></o:encrypted></message>`
	out := encryptedElement(raw)
	var wire encryptedWire
	if err := xml.Unmarshal([]byte(out), &wire); err != nil || wire.Header.SID != 42 || len(wire.Header.Keys) != 1 {
		t.Fatal(out, err)
	}
	if _, err := validateWebCiphertext(out); err != nil {
		t.Fatal(err)
	}
}
