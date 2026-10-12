// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
)

func TestEWSContactNamesAreNotCompany(t *testing.T) {
	ctx, store, user, eas, _, book := syncFixture(t)
	card, err := store.UpsertAddressObject(ctx, &storage.AddressObject{AddressBookID: book, UID: "person", HrefName: "person.vcf", Data: "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Иван Петров\r\nN:Петров;Иван;Иванович;;\r\nORG:Example Company\r\nTEL;TYPE=WORK:+100\r\nTEL;TYPE=CELL:+200\r\nEND:VCARD\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	h := &ewsHandler{store: store, ms: eas.ms}
	listed, err := h.findContactItems(ctx, book)
	if err != nil {
		t.Fatal(err)
	}
	got, err := h.getItem(ctx, user, `<GetItem><ItemId Id="`+card.ID+`"/></GetItem>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, xml := range []string{listed, got} {
		requireXML(t, xml)
		for _, field := range []string{"<t:FileAs>Иван Петров</t:FileAs>", "<t:GivenName>Иван</t:GivenName>", "<t:MiddleName>Иванович</t:MiddleName>", "<t:LastName>Петров</t:LastName>", "<t:Surname>Петров</t:Surname>", "<t:CompanyName>Example Company</t:CompanyName>", `Key="BusinessPhone">+100`, `Key="MobilePhone">+200`} {
			if !strings.Contains(xml, field) {
				t.Fatalf("missing %s", field)
			}
		}
		if strings.Index(xml, "<t:Surname>") < strings.Index(xml, "<t:CompanyName>") {
			t.Fatal("invalid EWS property order")
		}
	}
	if extractAttr(got, "ItemId", "ChangeKey") == card.ETag {
		t.Fatal("old clients need a new mapping ChangeKey")
	}
}
