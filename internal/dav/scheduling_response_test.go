// SPDX-License-Identifier: Apache-2.0
package dav

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestSchedulingResponseMediaTypes(t *testing.T) {
	for _, method := range []string{"GET", "PROPFIND", "REPORT"} {
		for _, contentType := range []string{"", "text/calendar; charset=utf-8", "application/xml; charset=utf-8"} {
			for _, status := range []int{http.StatusOK, http.StatusBadRequest} {
				t.Run(method+"/"+contentType+"/"+http.StatusText(status), func(t *testing.T) {
					body := []byte("<script>alert('request data')</script>\x00\xff")
					h := calendarScheduling(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if contentType != "" {
							w.Header().Set("Content-Type", contentType)
						}
						// Exercise implicit headers as well as explicit error responses.
						if status != http.StatusOK {
							w.WriteHeader(status)
						}
						_, _ = w.Write(body)
					}))
					r := httptest.NewRequest(method, calPrincipal("alice@example.com")+"calendar/test.ics", strings.NewReader(""))
					r = r.WithContext(withUser(r.Context(), &storage.User{Email: "alice@example.com"}))
					out := httptest.NewRecorder()
					h.ServeHTTP(out, r)
					wantType := contentType
					if wantType == "" {
						wantType = "text/plain; charset=utf-8"
					}
					headers := out.Result().Header
					if out.Code != status || headers.Get("Content-Type") != wantType || headers.Get("X-Content-Type-Options") != "nosniff" {
						t.Fatalf("unsafe or changed response: status=%d headers=%v", out.Code, headers)
					}
					if !bytes.Equal(out.Body.Bytes(), body) {
						t.Fatal("protocol bytes changed")
					}
				})
			}
		}
	}
}
