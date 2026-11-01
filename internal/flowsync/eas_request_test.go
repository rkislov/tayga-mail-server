package flowsync

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type requestReadFailure struct{ reads int }

func (r *requestReadFailure) Read([]byte) (int, error) {
	r.reads++
	return 0, io.ErrUnexpectedEOF
}

func TestEASRequestBoundedRead(t *testing.T) {
	// Sync must retain its 8 MiB ceiling even though compose commands allow
	// larger MIME. Unknown Content-Length must not bypass the bounded read.
	for _, known := range []bool{false, true} {
		for _, extra := range []int{0, 1} {
			data := bytes.Repeat([]byte{'x'}, (8<<20)+extra)
			r := httptest.NewRequest("POST", "/", bytes.NewReader(data))
			if !known {
				r.ContentLength = -1
			}
			trace := &requestTrace{}
			r = r.WithContext(context.WithValue(r.Context(), traceKey{}, trace))
			w := httptest.NewRecorder()
			body, ok := readEASRequest(w, r, "Sync")
			if extra == 0 {
				if !ok || !bytes.Equal(body, data) || trace.RequestBytes != int64(len(data)) {
					t.Fatal("request at the exact limit was truncated or rejected")
				}
			} else if ok || w.Code != 413 || trace.Result != "request_too_large" || body != nil {
				t.Fatal("oversized sync request was accepted")
			}
			if !known && trace.RequestBytes != int64(len(data)) {
				t.Fatal("unknown-length request was not counted")
			}
		}
	}
	for _, cmd := range []string{"SendMail", "smartreply", "SMARTFORWARD"} {
		source := &requestReadFailure{}
		r := httptest.NewRequest("POST", "/", source)
		r.ContentLength = easRequestLimit(cmd) + 1
		trace := &requestTrace{}
		r = r.WithContext(context.WithValue(r.Context(), traceKey{}, trace))
		w := httptest.NewRecorder()
		if _, ok := readEASRequest(w, r, cmd); ok || w.Code != http.StatusRequestEntityTooLarge || source.reads != 0 || trace.RequestBytes != 0 || trace.Result != "request_too_large" {
			t.Fatal("known oversized compose body was read or misclassified")
		}
	}
}

func TestEASRequestReadFailureDiagnostics(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		r := httptest.NewRequest("POST", "/", io.MultiReader(strings.NewReader("partial"), &requestReadFailure{}))
		trace := &requestTrace{}
		ctx, cancel := context.WithCancel(context.WithValue(r.Context(), traceKey{}, trace))
		if cancelled {
			cancel()
		}
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()
		body, ok := readEASRequest(w, r, "SmartReply")
		cancel()
		want := "request_read_failed"
		if cancelled {
			want = "request_cancelled"
		}
		if ok || body != nil || w.Code != http.StatusBadRequest || trace.Result != want || trace.RequestBytes != 7 {
			t.Fatalf("read failure misclassified: HTTP=%d trace=%+v", w.Code, trace)
		}
	}
}
