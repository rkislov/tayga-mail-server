package migrate

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDAVCertificateExceptionIsPerJob(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer srv.Close()
	strict, err := davHTTPClient(srv.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	insecure, err := davHTTPClient(srv.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	if res, e := strict.Get(srv.URL); e == nil {
		res.Body.Close()
		t.Fatal("untrusted certificate accepted by default")
	}
	res, e := insecure.Get(srv.URL)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res, e := strict.Get(srv.URL); e == nil {
		res.Body.Close()
		t.Fatal("exception leaked between jobs")
	}
	redirected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, srv.URL, 302) }))
	defer redirected.Close()
	c, _ := davHTTPClient(redirected.URL, true)
	if res, e := c.Get(redirected.URL); e == nil {
		res.Body.Close()
		t.Fatal("cross-origin redirect accepted")
	}
}
