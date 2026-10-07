package spam

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRspamdCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkv2" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("From") != "a@ex.com" {
			t.Errorf("from header %q", r.Header.Get("From"))
		}
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(`{"score":12.5,"required_score":15,"action":"add header","symbols":{"TEST":{}}}`))
	}))
	defer srv.Close()

	c := NewRspamd(srv.URL, "", 0)
	res, err := c.Check(context.Background(), Meta{From: "a@ex.com", To: "b@ex.com"}, []byte("From: a\r\n\r\nx"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Score != 12.5 || res.Action != "add header" || len(res.Symbols) != 1 {
		t.Fatalf("%+v", res)
	}
	if MapAction(Config{FollowRspamd: true}, res) != ActionTag {
		t.Fatal(MapAction(Config{FollowRspamd: true}, res))
	}
	if MapAction(Config{RejectAbove: 10}, res) != ActionReject {
		t.Fatal("threshold reject")
	}
	if MapAction(Config{QuarantineAbove: 10, RejectAbove: 20}, res) != ActionQuarantine {
		t.Fatal("threshold quarantine")
	}
}
