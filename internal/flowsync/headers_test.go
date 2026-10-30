package flowsync

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

type headerCountingReader struct {
	io.Reader
	read int
}

func (r *headerCountingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	return n, err
}

func TestHeadersStopBeforeLargeBody(t *testing.T) {
	header := "Subject: =?UTF-8?B?0KLQtdGB0YI=?=\r\n continuation\r\nDate: Mon, 19 Oct 2026 12:34:56 +0300\r\n\r\n"
	r := &headerCountingReader{Reader: io.MultiReader(strings.NewReader(header), bytes.NewReader(make([]byte, 8<<20)))}
	got := parseMsgHeaders(r)
	if got.Subject != "Тест continuation" || got.SentDate.UTC().Format(time.RFC3339) != "2026-10-19T09:34:56Z" {
		t.Fatalf("headers lost: %+v", got)
	}
	if r.read > 64<<10 {
		t.Fatalf("read %d bytes for %d bytes of headers", r.read, len(header))
	}
}

func TestIndexedAndArchivedHeaders(t *testing.T) {
	ms := mailstore.New(t.TempDir())
	msg := &storage.Message{Subject: "Indexed subject", DateHdr: "Mon, 19 Oct 2026 12:34:56 +0300", FilePath: "not-in-local-cache"}
	got := indexedMsgHeaders(ms, msg)
	if got.Subject != msg.Subject || got.SentDate.IsZero() {
		t.Fatalf("indexed headers depend on the file: %+v", got)
	}
	path, _, err := ms.Deliver("user@example.com", "Archive", []byte("Subject: Archived\r\nDate: Mon, 19 Oct 2026 12:34:56 +0300\r\n\r\nbody"))
	if err != nil {
		t.Fatal(err)
	}
	got = indexedMsgHeaders(ms, &storage.Message{FilePath: path})
	if got.Subject != "Archived" || got.SentDate.IsZero() {
		t.Fatalf("legacy gzip fallback failed: %+v", got)
	}
}

func BenchmarkEWSIndexedHeaders(b *testing.B) {
	msg := &storage.Message{Subject: "Indexed subject", DateHdr: "Mon, 19 Oct 2026 12:34:56 +0300"}
	for b.Loop() {
		indexedMsgHeaders(nil, msg)
	}
}
