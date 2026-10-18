package xmpp

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func TestResumeReplaysOnlyUnacknowledgedAndPreservesCounters(t *testing.T) {
	srv, _, u := testServer(t)
	var initial, resumed bytes.Buffer
	old := &Session{srv: srv, user: u, authed: true, bound: true, JID: ParseJID(u.Email + "/phone"), bw: bufio.NewWriter(&initial)}
	old.sm = streamManagement{enabled: true, resumable: true, token: "test-token", received: 9}
	old.pepInterest = map[string]bool{"eu.siacs.conversations.axolotl.devicelist": true}
	old.pepCaps = "https://monal-im.org#caps"
	for _, body := range []string{"one", "two", "three"} {
		if err := old.writeString(`<message><body>` + body + `</body></message>`); err != nil {
			t.Fatal(err)
		}
	}
	old.retainStream()
	fresh := &Session{srv: srv, user: u, authed: true, JID: ParseJID(u.Email), bw: bufio.NewWriter(&resumed)}
	start := xml.StartElement{Name: xml.Name{Space: smNamespace, Local: "resume"}, Attr: []xml.Attr{{Name: xml.Name{Local: "previd"}, Value: "test-token"}, {Name: xml.Name{Local: "h"}, Value: "1"}}}
	if err := fresh.resumeStream(start); err != nil {
		t.Fatal(err)
	}
	got := resumed.String()
	if strings.Contains(got, "<body>one") || !strings.Contains(got, "<body>two") || !strings.Contains(got, "<body>three") || !strings.Contains(got, "h='9'") {
		t.Fatal(got)
	}
	if fresh.sm.sent != 3 || fresh.sm.acknowledged != 1 || len(fresh.sm.pending) != 2 || !fresh.bound {
		t.Fatalf("bad restored state %+v", fresh.sm)
	}
	if len(srv.hub.Sessions(u.Email)) != 1 {
		t.Fatal("resumed session not routed")
	}
	if !fresh.pepInterest["eu.siacs.conversations.axolotl.devicelist"] || fresh.pepCaps != old.pepCaps {
		t.Fatal("resumed stream lost PEP notification interests")
	}
	fresh.sm.ack(3)
	if len(fresh.sm.pending) != 0 || fresh.sm.bytes != 0 {
		t.Fatal("acknowledged replay retained")
	}
}

func TestResumeRejectsForeignExpiredAndInvalidCounters(t *testing.T) {
	srv, _, u := testServer(t)
	state := resumableStream{userID: u.ID, jid: ParseJID(u.Email + "/phone"), expires: time.Now().Add(time.Minute), state: streamManagement{enabled: true, resumable: true, token: "secret", sent: 1, pending: []string{"<message/>"}}}
	for _, tc := range []struct {
		user    *storage.User
		expired bool
		h       string
		want    string
	}{
		{&storage.User{ID: "other"}, false, "0", "item-not-found"},
		{u, true, "0", "item-not-found"},
		{u, false, "2", "bad-request"},
		{u, false, "-1", "bad-request"},
	} {
		srv.resumable = map[string]resumableStream{"secret": state}
		if tc.expired {
			r := srv.resumable["secret"]
			r.expires = time.Now().Add(-time.Second)
			srv.resumable["secret"] = r
		}
		var out bytes.Buffer
		s := &Session{srv: srv, user: tc.user, authed: true, bw: bufio.NewWriter(&out)}
		start := xml.StartElement{Attr: []xml.Attr{{Name: xml.Name{Local: "previd"}, Value: "secret"}, {Name: xml.Name{Local: "h"}, Value: tc.h}}}
		if err := s.resumeStream(start); err != nil {
			t.Fatal(err)
		}
		if s.bound || !strings.Contains(out.String(), tc.want) {
			t.Fatal(out.String())
		}
	}
}
