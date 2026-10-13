package httpapi

import (
	"bytes"
	"encoding/base64"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOutgoingAttachmentRoundTrip(t *testing.T) {
	data := bytes.Repeat([]byte("pdf-test"), 200000)
	raw := buildOutgoingMIME("sender@example.com", "Sender", []string{"user@example.com"}, nil, nil, "test", "body", "<p>body</p>", "<test@example.com>")
	got, err := attachOutgoingFiles(raw, []outgoingAttachment{{Name: "Билет.pdf", Data: base64.StdEncoding.EncodeToString(data)}})
	if err != nil {
		t.Fatal(err)
	}
	parsed := parseMIMEMessage(got)
	if len(parsed.Attachments) != 1 || !bytes.Equal(parsed.Attachments[0].Data, data) || parsed.Attachments[0].Filename != "Билет.pdf" {
		t.Fatal("attachment round trip failed")
	}
}
func TestStreamUploadLimitPreservesDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	os.WriteFile(path, []byte("old"), 0600)
	r := httptest.NewRequest("PUT", "/", bytes.NewReader([]byte("new")))
	r.ContentLength = maxWebUploadBytes + 1
	if _, err := streamUpload(httptest.NewRecorder(), r, path); err == nil {
		t.Fatal("accepted oversize")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "old" {
		t.Fatal("overwritten")
	}
	r = httptest.NewRequest("PUT", "/", bytes.NewReader([]byte("new")))
	if _, err := streamUpload(httptest.NewRecorder(), r, path); err != nil {
		t.Fatal(err)
	}
}
