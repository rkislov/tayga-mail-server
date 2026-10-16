// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func TestAppleMailHTMLPreviewFetchAndInlineImage(t *testing.T) {
	ctx, st, u, h, dev, _ := syncFixture(t)
	if err := (&ewsHandler{store: st, ms: h.ms}).ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	mb, err := st.GetMailbox(ctx, u.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	html := `<html><head><style>td { padding: 24px; color: #123456; }</style></head><body><table><tr><td>Привет &amp; мир</td><td><img src="cid:logo@example.com"></td></tr></table>` + strings.Repeat(`<p>Текст с разметкой</p>`, 3000) + `</body></html>`
	picture := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 255, 2}
	raw := []byte("From: sender@example.com\r\nSubject: HTML layout\r\nMIME-Version: 1.0\r\nContent-Type: multipart/related; boundary=related\r\n\r\n--related\r\nContent-Type: multipart/alternative; boundary=alternative\r\n\r\n--alternative\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + strings.Repeat("Preview ", 200) + "\r\n--alternative\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n" + base64.StdEncoding.EncodeToString([]byte(html)) + "\r\n--alternative--\r\n--related\r\nContent-Type: image/png; name=logo.png\r\nContent-ID: <logo@example.com>\r\nContent-Disposition: inline; filename=logo.png\r\nContent-Transfer-Encoding: base64\r\n\r\n" + base64.StdEncoding.EncodeToString(picture) + "\r\n--related--\r\n")
	rel, size, err := h.ms.Deliver(u.Email, "INBOX", raw)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: rel, Size: size, InternalDate: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	call := func(command, body string) (*protocolNode, []byte) {
		t.Helper()
		wire, err := encodeWire(body)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd="+command+"&DeviceId="+dev.DeviceID, bytes.NewReader(wire))
		r.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
		r.Header.Set("MS-ASProtocolVersion", "16.1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r.WithContext(withUser(ctx, u)))
		if w.Code != 200 {
			t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
		}
		decoded, err := decodeWire(w.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		doc, err := parseProtocolXML(decoded)
		if err != nil {
			t.Fatal(err)
		}
		return doc, w.Body.Bytes()
	}
	sync := func(key, extra string) (*protocolNode, []byte) {
		return call("Sync", `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>`+key+`</SyncKey><CollectionId>`+mb.ID+`</CollectionId>`+extra+`</Collection></Collections></Sync>`)
	}
	previewOptions := `<Options><BodyPreference xmlns="AirSyncBase:"><Type>1</Type><TruncationSize>500</TruncationSize></BodyPreference></Options>`
	initial, _ := sync("0", previewOptions)
	preview, _ := sync(initial.find("Collection").value("SyncKey"), "")
	if preview.find("Body").value("Type") != "1" || preview.find("Body").value("Truncated") != "1" || preview.find("ApplicationData").value("NativeBodyType") != "2" {
		t.Fatal("plain preview must advertise native HTML")
	}
	key := preview.find("Collection").value("SyncKey")
	extras := `<GetChanges>0</GetChanges><Options><BodyPreference xmlns="AirSyncBase:"><Type>2</Type></BodyPreference></Options><Commands><Fetch><ServerId>` + msg.ID + `</ServerId></Fetch></Commands>`
	full, _ := sync(key, extras)
	assertHTML := func(doc *protocolNode) {
		t.Helper()
		body := doc.find("Body")
		if body == nil || body.value("Type") != "2" || body.value("Truncated") != "0" || body.value("Data") != html {
			t.Fatal("full HTML or layout was changed")
		}
		attachment := doc.find("Attachment")
		if attachment == nil || attachment.value("ContentId") != "logo@example.com" || attachment.value("IsInline") != "1" || attachment.value("FileReference") != msg.ID+":0" {
			t.Fatal("inline image is not linked to HTML")
		}
	}
	assertHTML(full)
	// A retry of a successful pre-upgrade Fetch must acquire the missing metadata,
	// preserve its sync key, and not duplicate Fetch responses.
	state, err := loadCollectionState(ctx, st, u.ID, dev.ID, mb.ID)
	if err != nil {
		t.Fatal(err)
	}
	cached, err := parseProtocolXML([]byte(state.Response))
	if err != nil {
		t.Fatal(err)
	}
	app := cached.find("ApplicationData")
	var kept []*protocolNode
	for _, child := range app.Children {
		if child.Name.Local != "NativeBodyType" {
			kept = append(kept, child)
		}
	}
	app.Children = kept
	state.Response = cached.render()
	if err := saveCollectionState(ctx, st, u.ID, dev.ID, mb.ID, state); err != nil {
		t.Fatal(err)
	}
	replayed, _ := sync(key, extras)
	assertHTML(replayed)
	if len(replayed.find("Responses").Children) != 1 || replayed.find("ApplicationData").value("NativeBodyType") != "2" || replayed.find("Collection").value("SyncKey") != state.Key {
		t.Fatal("invalid upgraded Fetch replay")
	}
	ioRequest := func(options string) string {
		return `<ItemOperations xmlns="ItemOperations:"><Fetch><Store>Mailbox</Store><ServerId xmlns="AirSync:">` + msg.ID + `</ServerId><Options>` + options + `</Options></Fetch></ItemOperations>`
	}
	item, _ := call("ItemOperations", ioRequest(`<BodyPreference xmlns="AirSyncBase:"><Type>2</Type></BodyPreference>`))
	assertHTML(item)
	attachment, wire := call("ItemOperations", `<ItemOperations xmlns="ItemOperations:"><Fetch><Store>Mailbox</Store><FileReference xmlns="AirSyncBase:">`+msg.ID+`:0</FileReference></Fetch></ItemOperations>`)
	data, err := base64.StdEncoding.DecodeString(attachment.find("Properties").value("Data"))
	if err != nil || !bytes.Equal(data, picture) || !bytes.Contains(wire, picture) {
		t.Fatal("inline image bytes corrupted")
	}
	// MIME is sent as original opaque bytes, never as a base64 string on the wire.
	mimeOptions := `<BodyPreference xmlns="AirSyncBase:"><Type>4</Type></BodyPreference><MIMESupport xmlns="AirSync:">2</MIMESupport>`
	item, wire = call("ItemOperations", ioRequest(mimeOptions))
	data, err = base64.StdEncoding.DecodeString(item.find("Body").value("Data"))
	if err != nil || item.find("Body").value("Type") != "4" || !bytes.Equal(data, raw) || !bytes.Contains(wire, raw) {
		t.Fatal("MIME round trip corrupted")
	}
	mime, _ := sync(state.Key, `<GetChanges>0</GetChanges><Options>`+mimeOptions+`</Options><Commands><Fetch><ServerId>`+msg.ID+`</ServerId></Fetch></Commands>`)
	if mime.find("Body").value("Type") != "4" || mime.find("Body").value("Data") != base64.StdEncoding.EncodeToString(raw) {
		t.Fatal("Sync Fetch ignored MIME preference")
	}
}

func TestBodyNegotiationAndBinaryMIME(t *testing.T) {
	largeHTML := "<html>" + strings.Repeat("x", 1200000) + "</html>"
	large, _, _, truncated := preferredBody(messageContent{HTML: largeHTML}, syncPreferences{Bodies: []bodyPreference{{Type: "2", Size: 2 << 20, HasSize: true}}})
	if truncated || large != largeHTML {
		t.Fatal("must honor the requested 2 MiB limit instead of silently capping it at 1 MiB")
	}
	content := messageContent{Text: "plain", HTML: `<b>html</b>`, MIME: []byte{'a', 0, 255, 13, 10}}
	for _, support := range []int{0, 1, 2} {
		p := syncPreferences{MIMESupport: support, Bodies: []bodyPreference{{Type: "1"}, {Type: "2"}, {Type: "4"}}}
		body, kind, size, truncated := preferredBody(content, p)
		want := "2"
		if support == 2 {
			want = "4"
		}
		if kind != want || truncated {
			t.Fatal("wrong negotiated body type")
		}
		if kind == "4" {
			encoded := `<Properties xmlns="ItemOperations:">` + renderMailBody(syncAdd{Body: body, BodyType: kind, EstimatedSize: size}) + `</Properties>`
			wire, err := encodeWire(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(wire, content.MIME) {
				t.Fatal("raw MIME not preserved")
			}
			xml, err := decodeWire(wire)
			if err != nil {
				t.Fatal(err)
			}
			if extractTag(string(xml), "Data") != base64.StdEncoding.EncodeToString(content.MIME) {
				t.Fatal("binary MIME corrupted")
			}
		}
	}
	for _, value := range []string{"-1", "3", "invalid"} {
		doc, err := parseProtocolXML([]byte(`<Fetch><Options><MIMESupport>` + value + `</MIMESupport></Options></Fetch>`))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := preferencesFrom(doc.find("Fetch"), syncPreferences{}); err == nil {
			t.Fatal("invalid MIME support accepted")
		}
	}
}
