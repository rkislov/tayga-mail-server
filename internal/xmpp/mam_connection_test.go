package xmpp

import (
	"context"
	"encoding/xml"
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
)

type mamWirePage struct {
	IDs      []string
	First    string
	Last     string
	Count    int
	Complete bool
}

func readMAMWirePage(t *testing.T, c *resumeWireClient, id, resource string) mamWirePage {
	return readArchiveWirePage(t, c, id, resource, "alice@example.com")
}
func readArchiveWirePage(t *testing.T, c *resumeWireClient, id, resource, archive string) mamWirePage {
	t.Helper()
	var page mamWirePage
	for {
		e := c.next()
		if e.XMLName == (xml.Name{Space: smNamespace, Local: "r"}) {
			continue
		}
		switch e.XMLName.Local {
		case "message":
			var msg struct {
				Result struct {
					ID      string `xml:"id,attr"`
					QueryID string `xml:"queryid,attr"`
					Forward struct {
						Message struct {
							IDs []struct {
								By string `xml:"by,attr"`
								ID string `xml:"id,attr"`
							} `xml:"urn:xmpp:sid:0 stanza-id"`
						} `xml:"jabber:client message"`
					} `xml:"urn:xmpp:forward:0 forwarded"`
				} `xml:"urn:xmpp:mam:2 result"`
			}
			if err := xml.Unmarshal([]byte("<message>"+e.Inner+"</message>"), &msg); err != nil {
				t.Fatal(err)
			}
			r := msg.Result
			if r.QueryID != id || r.ID == "" || len(r.Forward.Message.IDs) != 1 || r.Forward.Message.IDs[0].By != archive || r.Forward.Message.IDs[0].ID != r.ID {
				t.Fatalf("invalid MAM result/forwarded namespace/stable ID: %+v", r)
			}
			page.IDs = append(page.IDs, r.ID)
		case "iq":
			if e.ID != id {
				t.Fatalf("unexpected IQ: %+v", e)
			}
			var result struct {
				Fin *struct {
					Complete bool `xml:"complete,attr"`
					Set      struct {
						First string `xml:"first"`
						Last  string `xml:"last"`
						Count int    `xml:"count"`
					} `xml:"http://jabber.org/protocol/rsm set"`
				} `xml:"urn:xmpp:mam:2 fin"`
			}
			if err := xml.Unmarshal([]byte("<iq>"+e.Inner+"</iq>"), &result); err != nil || result.Fin == nil {
				t.Fatalf("missing MAM completion for %s/%s: %+v %v", resource, id, e, err)
			}
			page.First, page.Last, page.Count, page.Complete = result.Fin.Set.First, result.Fin.Set.Last, result.Fin.Set.Count, result.Fin.Complete
			if len(page.IDs) > 0 && (page.First != page.IDs[0] || page.Last != page.IDs[len(page.IDs)-1]) {
				t.Fatalf("RSM bounds differ from wire results: %+v", page)
			}
			return page
		default:
			t.Fatalf("unexpected stanza: %+v", e)
		}
	}
}

func TestMAMWirePagesTwoDevicesAndReconnect(t *testing.T) {
	srv, store, u := testServer(t)
	srv.authn = auth.NewLayer(store, config.LDAPConfig{}, config.MFAConfig{}, config.OIDCConfig{})
	hash, err := srv.authn.Hasher.Hash("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateUserPassword(context.Background(), u.ID, hash); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	phone := newResumeWireClient(t, srv, ln, "phone")
	desktop := newResumeWireClient(t, srv, ln, "desktop")
	phone.send(`<enable xmlns='urn:xmpp:sm:3' resume='true'/>`)
	token := phone.want("enabled", "").ID

	// Larger than the server page cap, with timestamp ties and an unrelated
	// archive. Use the actual XML stream and independent resource sessions.
	var expected []string
	stamp := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 257; i++ {
		id := fmt.Sprintf("archive-%03d", i)
		expected = append(expected, id)
		_, err := srv.db.Exec(`INSERT INTO xmpp_mam(id,owner_bare_jid,with_bare_jid,stanza_id,stanza,created_at) VALUES(?,?,?,?,?,?)`, id, u.Email, "bob@example.com", id, `<message from='bob@example.com/phone' to='alice@example.com'><body>fixture</body></message>`, stamp.Add(time.Duration(i/4)*time.Millisecond))
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := srv.archiveMAM(context.Background(), "other@example.com", "bob@example.com", "private-cursor", `<message><body>private fixture</body></message>`); err != nil {
		t.Fatal(err)
	}
	query := func(c *resumeWireClient, id, resource, rsm string) mamWirePage {
		t.Helper()
		c.send(`<iq type='set' id='` + id + `'><query xmlns='urn:xmpp:mam:2' queryid='` + id + `'><set xmlns='http://jabber.org/protocol/rsm'>` + rsm + `</set></query></iq>`)
		return readMAMWirePage(t, c, id, resource)
	}
	check := func(page mamWirePage, want []string, total int, complete bool) {
		t.Helper()
		if !reflect.DeepEqual(page.IDs, want) || page.Count != total || page.Complete != complete {
			t.Fatalf("page = %+v; want %v, count %d, complete %t", page, want, total, complete)
		}
	}
	first := query(phone, "phone-first", "phone", `<max>500</max>`)
	check(first, expected[:100], 257, false)
	last := query(desktop, "desktop-last", "desktop", `<max>100</max><before/>`)
	check(last, expected[157:], 257, false)

	// A new live message is committed and delivered between page requests.
	live, liveID, err := srv.prepareArchivedMessage(context.Background(), `<message from='bob@example.com/phone' to='alice@example.com/phone' type='chat' id='live'><body>arrived during history sync</body></message>`)
	if err != nil || !srv.hub.Route(u.Email+"/phone", []byte(live)) {
		t.Fatalf("live message: %v", err)
	}
	phone.want("message", "live")
	if carbon := desktop.want("message", ""); !strings.Contains(carbon.Inner, "<received") || !strings.Contains(carbon.Inner, liveID) {
		t.Fatalf("missing live carbon: %+v", carbon)
	}
	phone.send(`<a xmlns='urn:xmpp:sm:3' h='102'/><r xmlns='urn:xmpp:sm:3'/>`)
	phone.want("a", "") // 100 results, one fin, one live message acknowledged
	phone.conn.Close()
	select {
	case <-phone.done:
	case <-time.After(4 * time.Second):
		t.Fatal("phone did not disconnect")
	}
	resumed := newResumeWireClient(t, srv, ln, "")
	resumed.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='102'/>`)
	resumed.want("resumed", "")
	middle := query(resumed, "phone-middle", "phone", `<max>100</max><after>`+first.Last+`</after>`)
	check(middle, expected[100:200], 258, false)
	end := query(resumed, "phone-end", "phone", `<max>100</max><after>`+middle.Last+`</after>`)
	check(end, append(append([]string{}, expected[200:]...), liveID), 258, true)
	older := query(desktop, "desktop-middle", "desktop", `<max>100</max><before>`+last.First+`</before>`)
	check(older, expected[57:157], 258, false)
	oldest := query(desktop, "desktop-first", "desktop", `<max>100</max><before>`+older.First+`</before>`)
	check(oldest, expected[:57], 258, true)
	count := query(desktop, "count", "desktop", `<max>0</max>`)
	check(count, nil, 258, false)
	if count.First != "" || count.Last != "" {
		t.Fatal("count-only query included page boundaries")
	}
	// Neither device may receive another device's history via Carbons.
	for _, c := range []*resumeWireClient{resumed, desktop} {
		c.send(`<iq type='get' id='barrier'><ping xmlns='urn:xmpp:ping'/></iq>`)
		c.want("iq", "barrier")
	}
	desktop.send(`<iq type='set' id='foreign'><query xmlns='urn:xmpp:mam:2'><set xmlns='http://jabber.org/protocol/rsm'><after>private-cursor</after></set></query></iq>`)
	if e := desktop.want("iq", "foreign"); !strings.Contains(e.Inner, "item-not-found") {
		t.Fatalf("foreign archive cursor accepted: %+v", e)
	}
}
