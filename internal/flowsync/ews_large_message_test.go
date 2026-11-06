package flowsync

import (
	"context"
	"encoding/base64"
	"github.com/tayga/tms/internal/storage"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEWSSingleMIMEDownloadAndBatchErrorTrace(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	mb, err := st.EnsureMailbox(ctx, u.ID, "INBOX", "")
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("Subject: XML <&> preservation\r\nContent-Type: application/octet-stream\r\n\r\n" + strings.Repeat("ABC", 128<<10))
	path, size, err := eas.ms.Deliver(u.Email, "INBOX", raw)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: path, Size: size, InternalDate: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	for _, batch := range []bool{false, true} {
		ids := `<t:ItemId Id="` + msg.ID + `"/>`
		if batch {
			ids += `<t:ItemId Id="missing-item"/>`
		}
		wire := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><s:Body><m:GetItem><m:ItemShape><t:BaseShape>IdOnly</t:BaseShape><t:IncludeMimeContent>true</t:IncludeMimeContent></m:ItemShape><m:ItemIds>` + ids + `</m:ItemIds></m:GetItem></s:Body></s:Envelope>`
		trace := &requestTrace{}
		r := httptest.NewRequest("POST", "/EWS/Exchange.asmx", strings.NewReader(wire)).WithContext(context.WithValue(withUser(ctx, u), traceKey{}, trace))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		doc, err := parseProtocolXML(w.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(doc.find("MimeContent").Text)
		if err != nil || string(data) != string(raw) {
			t.Fatal("MIME changed", err)
		}
		if batch && trace.Result != "ErrorItemNotFound" {
			t.Fatal("later batch error hidden by MIME optimization", trace.Result)
		}
		if !batch && trace.Result != "" {
			t.Fatal("single success trace changed", trace.Result)
		}
	}
}
