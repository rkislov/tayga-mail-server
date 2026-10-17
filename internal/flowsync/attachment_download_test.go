// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func TestAndroidAttachmentDownloadUsesBase64String(t *testing.T) {
	ctx, st, u, h, dev, _ := syncFixture(t)
	if err := (&ewsHandler{store: st, ms: h.ms}).ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	mb, err := st.GetMailbox(ctx, u.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, mime string
		size       int
	}{
		{"document.pdf", "application/pdf", 1300000},
		{"image.png", "image/png", 902108},
		{"binary.dat", "application/octet-stream", 256},
		{"empty.txt", "text/plain", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := make([]byte, tc.size)
			for i := range data {
				data[i] = byte(i)
			}
			encoded := base64.StdEncoding.EncodeToString(data)
			raw := []byte(fmt.Sprintf("From: sender@example.com\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=test\r\n\r\n--test\r\nContent-Type: text/plain\r\n\r\nAttachment test\r\n--test\r\nContent-Type: %s\r\nContent-Disposition: attachment; filename=%s\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n--test--\r\n", tc.mime, tc.name, encoded))
			path, size, err := h.ms.Deliver(u.Email, "INBOX", raw)
			if err != nil {
				t.Fatal(err)
			}
			msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: path, Size: size, InternalDate: time.Now()})
			if err != nil {
				t.Fatal(err)
			}
			xml := `<ItemOperations xmlns="ItemOperations:"><Fetch><Store>Mailbox</Store><FileReference xmlns="AirSyncBase:">` + msg.ID + `:0</FileReference></Fetch></ItemOperations>`
			wire, err := encodeWire(xml)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=ItemOperations&DeviceId="+dev.DeviceID, bytes.NewReader(wire))
			r.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
			r.Header.Set("MS-ASProtocolVersion", "16.1")
			r.Header.Set("User-Agent", "Android-Mail/2026.09.28")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r.WithContext(withUser(ctx, u)))
			if w.Code != 200 || w.Header().Get("Content-Type") != "application/vnd.ms-sync.wbxml" {
				t.Fatalf("unexpected HTTP response %d", w.Code)
			}
			// Inspect the actual wire representation independently of decodeWire:
			// Data (page 20, tag 0x0c) must be STR_I containing base64, not OPAQUE.
			if len(data) > 0 {
				expected := append([]byte{0, 20, 0x4c, 0x03}, encoded...)
				expected = append(expected, 0, 1)
				if !bytes.Contains(w.Body.Bytes(), expected) {
					t.Fatal("attachment must be encoded as a base64 WBXML string")
				}
			}
			decoded, err := decodeWire(w.Body.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			doc, err := parseProtocolXML(decoded)
			if err != nil || doc.find("Fetch").value("Status") != "1" {
				t.Fatal("attachment fetch failed", err)
			}
			got, err := base64.StdEncoding.DecodeString(doc.find("Properties").value("Data"))
			if err != nil || !bytes.Equal(got, data) {
				t.Fatal("downloaded attachment bytes differ", err)
			}
			// File references remain scoped to the authenticated user's mailbox.
			other := *u
			other.ID = "other-user"
			denied, err := h.itemOperations(ctx, &other, []byte(xml))
			if err != nil || !strings.Contains(denied, `<Fetch><Status>6</Status>`) || strings.Contains(denied, "<Data>") {
				t.Fatal("attachment leaked across accounts", err)
			}
		})
	}
}
