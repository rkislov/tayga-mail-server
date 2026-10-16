// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func TestMailPreviewFetchDoesNotRestartSync(t *testing.T) {
	ctx, st, u, h, dev, _ := syncFixture(t)
	if err := (&ewsHandler{store: st, ms: h.ms}).ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	mb, err := st.GetMailbox(ctx, u.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	var messages []*storage.Message
	html := `<html><body><table><tr><td>` + strings.Repeat("HTML body ", 400) + `</td></tr></table></body></html>`
	for i := 0; i < 3; i++ {
		raw := []byte("From: sender@example.com\r\nSubject: Preview\r\nContent-Type: text/html; charset=utf-8\r\n\r\n" + html)
		path, size, e := h.ms.Deliver(u.Email, "INBOX", raw)
		if e != nil {
			t.Fatal(e)
		}
		msg, e := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: path, Size: size, InternalDate: time.Now()})
		if e != nil {
			t.Fatal(e)
		}
		messages = append(messages, msg)
	}
	preview := `<Options><FilterType>5</FilterType><MIMESupport>2</MIMESupport><BodyPreference xmlns="AirSyncBase:"><Type>1</Type><TruncationSize>500</TruncationSize></BodyPreference></Options>`
	request := func(key, extra string) string {
		return `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + mb.ID + `</CollectionId>` + extra + `</Collection></Collections></Sync>`
	}
	call := func(key, extra string) (*protocolNode, string) {
		t.Helper()
		out, e := h.syncCollections(ctx, u, dev, []byte(request(key, extra)))
		if e != nil {
			t.Fatal(e)
		}
		doc, e := parseProtocolXML([]byte(out))
		if e != nil {
			t.Fatal(e)
		}
		if doc.find("Collection").value("Status") != "1" {
			t.Fatalf("sync rejected: %s", out)
		}
		return doc, out
	}
	initial, _ := call("0", preview)
	key := initial.find("Collection").value("SyncKey")
	first, _ := call(key, `<WindowSize>1</WindowSize>`+preview)
	key = first.find("Collection").value("SyncKey")
	if first.find("MoreAvailable") == nil {
		t.Fatal("expected partial snapshot")
	}
	fetch := func(id, kind string) string {
		return `<GetChanges>0</GetChanges><Options><FilterType>5</FilterType><MIMESupport>2</MIMESupport><BodyPreference xmlns="AirSyncBase:"><Type>` + kind + `</Type></BodyPreference></Options><Commands><Fetch><ServerId>` + id + `</ServerId></Fetch></Commands>`
	}
	fetched, _ := call(key, fetch(messages[0].ID, "4"))
	if fetched.find("Collection").value("SyncKey") != key {
		t.Fatal("read-only Fetch advanced the Sync key")
	}
	if fetched.find("Fetch").value("Status") != "1" || fetched.find("Body").value("Type") != "4" {
		t.Fatal("MIME Fetch failed")
	}
	// A page response can be lost while a concurrent Fetch uses its input key.
	// The Fetch must preserve the page replay and not acknowledge unseen items.
	lostRequestKey := key
	second, secondXML := call(key, `<WindowSize>1</WindowSize>`+preview)
	call(lostRequestKey, fetch(messages[1].ID, "2"))
	_, replayed := call(lostRequestKey, `<WindowSize>1</WindowSize>`+preview)
	if replayed != secondXML {
		t.Fatal("Fetch overwrote a pending page replay")
	}
	key = second.find("Collection").value("SyncKey")
	third, _ := call(key, `<WindowSize>1</WindowSize>`+preview)
	key = third.find("Collection").value("SyncKey")
	if third.find("MoreAvailable") != nil {
		t.Fatal("snapshot failed to converge")
	}
	call(lostRequestKey, fetch(messages[0].ID, "4")) // read request queued two pages ago
	for _, kind := range []string{"2", "4", "1", "4"} {
		call(key, fetch(messages[0].ID, kind))
		idle, _ := call(key, preview)
		key = idle.find("Collection").value("SyncKey")
		if idle.find("Commands") != nil {
			t.Fatal("presentation change resent unchanged mail")
		}
	}
	estimate, _, err := h.itemEstimate(ctx, u, dev, []byte(`<GetItemEstimate xmlns="GetItemEstimate:"><Collections><Collection><SyncKey>`+key+`</SyncKey><CollectionId>`+mb.ID+`</CollectionId>`+preview+`</Collection></Collections></GetItemEstimate>`), false)
	if err != nil || !strings.Contains(estimate, "<Estimate>0</Estimate>") {
		t.Fatalf("false estimate: %s %v", estimate, err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 25*time.Millisecond)
	defer cancel()
	pong, err := h.ping(pingCtx, u, dev, []byte(`<Ping xmlns="Ping:"><HeartbeatInterval>60</HeartbeatInterval><Folders><Folder><Id>`+mb.ID+`</Id><Class>Email</Class></Folder></Folders></Ping>`))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("false Ping wakeup: %s %v", pong, err)
	}
	// Old read keys never authorize a mutation.
	denied, err := h.syncCollections(ctx, u, dev, []byte(request(lostRequestKey, `<GetChanges>0</GetChanges><Commands><Delete><ServerId>`+messages[0].ID+`</ServerId></Delete></Commands>`)))
	if err != nil || !strings.Contains(denied, "<Status>3</Status>") {
		t.Fatal("stale read key accepted a write")
	}
	if err = st.UpdateMessageFlags(ctx, messages[0].ID, `\Seen`); err != nil {
		t.Fatal(err)
	}
	changed, _ := call(key, preview)
	key = changed.find("Collection").value("SyncKey")
	if changed.find("Change") == nil || changed.find("Change").value("ServerId") != messages[0].ID || len(changed.find("Commands").Children) != 1 {
		t.Fatal("real read-state change was lost or resent other mail")
	}
	// A reconstructed handler must retain the stable snapshot.
	h = &easHandler{store: st, ms: h.ms}
	idle, _ := call(key, preview)
	if idle.find("Commands") != nil {
		t.Fatal("restart invalidated snapshot")
	}
}

func TestMailLegacySnapshotUpgradePreservesAcknowledgements(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "before_native_type", true: "with_native_type"}[native], func(t *testing.T) {
			ctx, st, u, h, dev, _ := syncFixture(t)
			if err := (&ewsHandler{store: st, ms: h.ms}).ensureStandardMailFolders(ctx, u); err != nil {
				t.Fatal(err)
			}
			mb, err := st.GetMailbox(ctx, u.ID, "INBOX")
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for i := 0; i < 3; i++ {
				path, size, e := h.ms.Deliver(u.Email, "INBOX", []byte("Subject: Legacy\r\n\r\nbody"))
				if e != nil {
					t.Fatal(e)
				}
				msg, e := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: path, Size: size, InternalDate: time.Now()})
				if e != nil {
					t.Fatal(e)
				}
				ids = append(ids, msg.ID)
			}
			prefs := syncPreferences{Filter: 5, Bodies: []bodyPreference{{Type: "1", Size: 500, HasSize: true}}, MIMESupport: 2}
			full, _, err := h.syncMailCollection(context.WithValue(context.WithValue(ctx, preferencesKey{}, prefs), mailSnapshotKey{}, true), u, mb.ID, "unused", false, nil)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := parseProtocolXML([]byte(full))
			if err != nil {
				t.Fatal(err)
			}
			old := map[string]string{}
			for _, item := range doc.find("Commands").Children {
				if !native {
					data := item.child("ApplicationData")
					var kept []*protocolNode
					for _, child := range data.Children {
						if child.Name.Local != "NativeBodyType" {
							kept = append(kept, child)
						}
					}
					data.Children = kept
				}
				if item.value("ServerId") != ids[2] {
					old[item.value("ServerId")] = digest(item.render() + prefs.hash())
				}
			}
			// One item was changed after acknowledgement, one has not been sent yet.
			if err := st.UpdateMessageFlags(ctx, ids[1], `\Seen`); err != nil {
				t.Fatal(err)
			}
			old["deleted-message"] = "old"
			oldKey := storage.NewID()
			fetchKey := storage.NewID()
			state := collectionState{Preferences: syncPreferences{Filter: 5, Bodies: []bodyPreference{{Type: "4"}}, MIMESupport: 2}, Key: fetchKey, PreviousKey: oldKey, Items: old, Response: `<Collection xmlns="AirSync:"><SyncKey>` + fetchKey + `</SyncKey><Status>1</Status><Responses><Fetch><Status>1</Status></Fetch></Responses></Collection>`}
			if err := saveCollectionState(ctx, st, u.ID, dev.ID, mb.ID, state); err != nil {
				t.Fatal(err)
			}
			request := `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + oldKey + `</SyncKey><CollectionId>` + mb.ID + `</CollectionId><Options><FilterType>5</FilterType><MIMESupport>2</MIMESupport><BodyPreference xmlns="AirSyncBase:"><Type>1</Type><TruncationSize>500</TruncationSize></BodyPreference></Options></Collection></Collections></Sync>`
			out, err := h.syncCollections(ctx, u, dev, []byte(request))
			if err != nil {
				t.Fatal(err)
			}
			got, err := parseProtocolXML([]byte(out))
			if err != nil {
				t.Fatal(err)
			}
			if got.find("Collection").value("Status") != "1" {
				t.Fatal("legacy Fetch key forced reset")
			}
			commands := got.find("Commands")
			if commands == nil || len(commands.Children) != 3 {
				t.Fatal("upgrade lost or repeated items")
			}
			for _, command := range commands.Children {
				id := command.value("ServerId")
				if id == ids[0] {
					t.Fatal("unchanged legacy item resent")
				}
				want := map[string]string{ids[1]: "Change", ids[2]: "Add", "deleted-message": "Delete"}[id]
				if command.Name.Local != want {
					t.Fatalf("expected %s got %s", want, command.Name.Local)
				}
			}
		})
	}
}
