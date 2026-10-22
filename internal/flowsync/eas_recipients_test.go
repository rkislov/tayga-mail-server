// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func TestEASResolveRecipientsWireAvailabilityAndPrivacy(t *testing.T) {
	ctx, st, u, h, _, ab := syncFixture(t)
	if err := st.PutSetting(ctx, "domain.gal."+u.DomainID, "true"); err != nil {
		t.Fatal(err)
	}
	room, err := st.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: u.DomainID, LocalPart: "room", DisplayName: "Room", Kind: "room"})
	if err != nil {
		t.Fatal(err)
	}
	cal, _ := st.GetCalendarByName(ctx, room.UserID, "default")
	_, err = st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, UID: "secret", HrefName: "secret.ics", Component: "VEVENT", Data: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:secret\r\nDTSTART:20261023T163000Z\r\nDTEND:20261023T170000Z\r\nSUMMARY:Secret appointment\r\nLOCATION:Secret room\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"guest@outside.example", "hidden@other.example"} {
		_, err = st.UpsertAddressObject(ctx, &storage.AddressObject{AddressBookID: ab, UID: email, HrefName: email + ".vcf", Data: buildVCARD(email, map[string]string{"FileAs": "Contact " + email, "Email1Address": email})})
		if err != nil {
			t.Fatal(err)
		}
	}
	dom, err := st.CreateDomain(ctx, u.TenantID, "other.example")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: dom.ID, LocalPart: "hidden", Email: "hidden@other.example", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	call := func(body string) *protocolNode {
		t.Helper()
		wire, err := encodeWire(body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(wire, []byte{0, 10, 0x45}) {
			t.Fatal("wrong ResolveRecipients code page")
		}
		r := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=ResolveRecipients&DeviceId=test&DeviceType=test", bytes.NewReader(wire)).WithContext(withUser(ctx, u))
		r.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		decoded, err := decodeWire(w.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(decoded), "Secret") {
			t.Fatal("event details leaked")
		}
		doc, err := parseProtocolXML(decoded)
		if err != nil {
			t.Fatal(err)
		}
		return doc.child("ResolveRecipients")
	}
	request := `<ResolveRecipients xmlns="ResolveRecipients:"><Options><Availability><StartTime>2026-10-23T16:00:00.000Z</StartTime><EndTime>2026-10-23T18:00:00.000Z</EndTime></Availability><CertificateRetrieval>1</CertificateRetrieval></Options><To>` + room.Email + `</To><To>` + u.Email + `</To><To>guest@outside.example</To><To>hidden@other.example</To><To>missing@outside.example</To></ResolveRecipients>`
	out := call(request)
	if out.value("Status") != "1" {
		t.Fatal(out.render())
	}
	expected := []struct{ address, status, availability, grid string }{{room.Email, "1", "1", "0200"}, {u.Email, "1", "1", "0000"}, {"guest@outside.example", "1", "163", ""}, {"hidden@other.example", "1", "163", ""}, {"missing@outside.example", "4", "", ""}}
	for i, want := range expected {
		response := out.Children[i+1]
		a := response.child("Recipient").child("Availability")
		if response.value("To") != want.address || response.value("Status") != want.status || a.value("Status") != want.availability || a.value("MergedFreeBusy") != want.grid {
			t.Fatal(response.render())
		}
		if response.find("Certificates") != nil {
			t.Fatal("certificate retrieval disabled")
		}
	}
	if err = st.PutSetting(ctx, "domain.gal."+u.DomainID, "false"); err != nil {
		t.Fatal(err)
	}
	out = call(request)
	if out.Children[1].value("Status") != "4" || out.Children[2].child("Recipient").child("Availability").value("MergedFreeBusy") != "0000" {
		t.Fatal(out.render())
	}
	for _, body := range []string{strings.Replace(request, "2026-10-23T18:00:00.000Z", "2026-10-23T15:00:00.000Z", 1), strings.Replace(request, "2026-10-23T18:00:00.000Z", "2027-10-23T18:00:00.000Z", 1), strings.Replace(request, "2026-10-23T16:00:00.000Z", "invalid", 1)} {
		if got := call(body); got.value("Status") != "5" {
			t.Fatal(got.render())
		}
	}
	// Omitting EndTime defaults to seven days, not a protocol failure.
	own := `<ResolveRecipients xmlns="ResolveRecipients:"><To>` + u.Email + `</To><Options><Availability><StartTime>2026-10-23T16:00:00Z</StartTime></Availability></Options></ResolveRecipients>`
	if grid := call(own).find("MergedFreeBusy").Text; len(grid) != 336 {
		t.Fatal(len(grid))
	}
	// No options is a valid recipient lookup.
	if got := call(`<ResolveRecipients xmlns="ResolveRecipients:"><To>` + u.Email + `</To></ResolveRecipients>`); got.value("Status") != "1" {
		t.Fatal(got.render())
	}
}

func TestEASResolveRecipientsUnavailableCalendarAndAmbiguity(t *testing.T) {
	ctx, st, u, h, _, ab := syncFixture(t)
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	_, err := st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, UID: "dense", HrefName: "dense.ics", Component: "VEVENT", Data: "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:dense\r\nDTSTART:20261023T160000Z\r\nDTEND:20261023T170000Z\r\nRRULE:FREQ=MINUTELY\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	raw := `<ResolveRecipients><To>` + u.Email + `</To><Options><Availability><StartTime>2026-10-23T16:00:00Z</StartTime></Availability></Options></ResolveRecipients>`
	out, err := h.resolveRecipients(ctx, u, []byte(raw))
	if err != nil || !strings.Contains(out, "<Status>162</Status>") || strings.Contains(out, "MergedFreeBusy") {
		t.Fatal(out, err)
	}
	for _, email := range []string{"alex1@outside.example", "alex2@outside.example"} {
		_, err = st.UpsertAddressObject(ctx, &storage.AddressObject{AddressBookID: ab, UID: email, HrefName: email + ".vcf", Data: buildVCARD(email, map[string]string{"FileAs": "Alex", "Email1Address": email})})
		if err != nil {
			t.Fatal(err)
		}
	}
	out, err = h.resolveRecipients(ctx, u, []byte(`<ResolveRecipients><To>Alex</To><Options><MaxAmbiguousRecipients>1</MaxAmbiguousRecipients></Options></ResolveRecipients>`))
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := parseProtocolXML([]byte(out))
	response := doc.find("Response")
	if response.value("Status") != "3" || response.value("RecipientCount") != "2" || strings.Count(out, "<Recipient>") != 1 {
		t.Fatal(out)
	}
}
func TestEASMergedAvailabilityBoundary(t *testing.T) {
	from := time.Date(2026, 10, 23, 16, 0, 0, 0, time.UTC)
	busy := []storage.BusyInterval{{Start: from.Add(-time.Hour), End: from}, {Start: from.Add(29 * time.Minute), End: from.Add(30 * time.Minute)}, {Start: from.Add(time.Hour), End: from.Add(2 * time.Hour)}}
	if got := mergedEASBusy(busy, from, from.Add(time.Hour)); got != "20" {
		t.Fatal(got)
	}
}

func TestEASGALSearchPagingAndDomainIsolation(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	if err := st.PutSetting(ctx, "domain.gal."+u.DomainID, "true"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bob", "ben"} {
		_, err := st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: name, Email: name + "@example.com", DisplayName: name, Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
	}
	call := func(query, span string) *protocolNode {
		t.Helper()
		wire, err := encodeWire(`<Search xmlns="Search:"><Store><Name>GAL</Name><Query>` + query + `</Query><Options><Range>` + span + `</Range></Options></Store></Search>`)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=Search&DeviceId=test", bytes.NewReader(wire)).WithContext(withUser(ctx, u))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		decoded, err := decodeWire(w.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		doc, _ := parseProtocolXML(decoded)
		return doc.find("Store")
	}
	first := call("b", "0-0")
	if first.value("Total") != "2" || first.value("Range") != "0-0" || first.find("EmailAddress").Text != "ben@example.com" {
		t.Fatal(first.render())
	}
	second := call("b", "1-1")
	if second.find("EmailAddress").Text != "bob@example.com" {
		t.Fatal(second.render())
	}
	if got := call("b", "2-1"); got.value("Status") != "2" {
		t.Fatal(got.render())
	}
	if err := st.PutSetting(ctx, "domain.gal."+u.DomainID, "false"); err != nil {
		t.Fatal(err)
	}
	if got := call("b", "0-9"); got.value("Total") != "0" || got.child("Result") != nil {
		t.Fatal(got.render())
	}
}
