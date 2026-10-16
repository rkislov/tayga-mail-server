// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestEmptyPingKeepsActiveSyncBinaryTransport(t *testing.T) {
	ctx, st, u, h, dev, ab := syncFixture(t)
	_, err := st.UpsertAddressObject(ctx, &storage.AddressObject{AddressBookID: ab, UID: "pending", HrefName: "pending.vcf", Data: "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Pending\r\nEND:VCARD\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	request := `<Ping xmlns="Ping:"><HeartbeatInterval>60</HeartbeatInterval><Folders><Folder><Id>` + ab + `</Id><Class>Contacts</Class></Folder></Folders></Ping>`
	wire, err := encodeWire(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range [][]byte{wire, nil} {
		r := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=Ping&DeviceId="+dev.DeviceID, bytes.NewReader(body))
		r.Header.Set("MS-ASProtocolVersion", "16.1")
		if len(body) > 0 {
			r.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r.WithContext(withUser(ctx, u)))
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/vnd.ms-sync.wbxml" {
			t.Fatalf("request bytes=%d: status=%d content-type=%q", len(body), w.Code, w.Header().Get("Content-Type"))
		}
		xml, err := decodeWire(w.Body.Bytes())
		if err != nil || extractTag(string(xml), "Status") != "2" || extractTag(string(xml), "Folder") != ab {
			t.Fatalf("empty Ping lost subscription or returned invalid WBXML: %s, %v", xml, err)
		}
	}
}
