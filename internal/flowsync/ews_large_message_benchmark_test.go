package flowsync

import (
	"bytes"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type ewsDiscardResponse struct {
	header http.Header
	bytes  int
}

func (w *ewsDiscardResponse) Header() http.Header         { return w.header }
func (w *ewsDiscardResponse) WriteHeader(int)             {}
func (w *ewsDiscardResponse) Write(p []byte) (int, error) { w.bytes += len(p); return len(p), nil }
func (w *ewsDiscardResponse) WriteString(s string) (int, error) {
	w.bytes += len(s)
	return len(s), nil
}

func BenchmarkEWSOpenMIME(b *testing.B) {
	for _, size := range []int{1 << 20, 32 << 20} {
		b.Run(fmt.Sprintf("%dMiB", size>>20), func(b *testing.B) {
			ctx, st, u, eas, _, _ := syncFixture(b)
			mb, err := st.EnsureMailbox(ctx, u.ID, "INBOX", "")
			if err != nil {
				b.Fatal(err)
			}
			raw := append([]byte("Subject: Large MIME\r\nContent-Type: multipart/mixed; boundary=b\r\n\r\n--b\r\nContent-Type: text/plain\r\n\r\nOpen test\r\n--b\r\nContent-Type: application/octet-stream\r\nContent-Transfer-Encoding: base64\r\nContent-Disposition: attachment; filename=large.bin\r\n\r\n"), bytes.Repeat([]byte("A"), size)...)
			raw = append(raw, []byte("\r\n--b--\r\n")...)
			path, n, err := eas.ms.Deliver(u.Email, "INBOX", raw)
			if err != nil {
				b.Fatal(err)
			}
			msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: path, Size: n, InternalDate: time.Now()})
			if err != nil {
				b.Fatal(err)
			}
			h := logRequests(slog.New(slog.NewTextHandler(io.Discard, nil)), "ews", &ewsHandler{store: st, ms: eas.ms}, true)
			wire := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><s:Body><m:GetItem><m:ItemShape><t:BaseShape>IdOnly</t:BaseShape><t:IncludeMimeContent>true</t:IncludeMimeContent></m:ItemShape><m:ItemIds><t:ItemId Id="` + msg.ID + `"/></m:ItemIds></m:GetItem></s:Body></s:Envelope>`
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r := httptest.NewRequest("POST", "/EWS/Exchange.asmx", strings.NewReader(wire)).WithContext(withUser(ctx, u))
				w := &ewsDiscardResponse{header: make(http.Header)}
				h.ServeHTTP(w, r)
				if w.bytes < len(raw) {
					b.Fatal("MIME response missing")
				}
			}
		})
	}
}
