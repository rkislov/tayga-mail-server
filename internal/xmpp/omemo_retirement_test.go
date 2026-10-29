package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
	"strings"
	"sync"
	"testing"
)

const retirementList = `<list xmlns="eu.siacs.conversations.axolotl"><device id="42"/><device id="7" label="remaining"/></list>`
const retirementBundle = `<bundle xmlns="eu.siacs.conversations.axolotl"><identityKey>fixture-key</identityKey></bundle>`
const retirementCipher = `<encrypted xmlns="eu.siacs.conversations.axolotl"><header sid="42"><key rid="7">YWJj</key><iv>AAAAAAAAAAAAAAAA</iv></header><payload>YWJj</payload></encrypted>`

func TestOMEMORetirementNativePersistence(t *testing.T) {
	s, st, u := testServer(t)
	ctx := context.Background()
	listNode, bundleNode := legacyOMEMO+".devicelist", legacyOMEMO+".bundles:42"
	for node, raw := range map[string]string{listNode: retirementList, bundleNode: retirementBundle} {
		if err := s.PublishWebOMEMO(ctx, u.Email, node, raw); err != nil {
			t.Fatal(err)
		}
	}
	var events, output bytes.Buffer
	peer := &Session{srv: s, JID: ParseJID(u.Email + "/second"), bw: bufio.NewWriter(&events), pepInterest: map[string]bool{listNode: true}}
	s.hub.Register(peer)
	session := &Session{srv: s, user: u, JID: ParseJID(u.Email + "/native"), bw: bufio.NewWriter(&output)}
	if err := s.RetireWebOMEMO(ctx, u.Email, 42, "wrong-key"); !errors.Is(err, ErrOMEMOKeyChanged) {
		t.Fatal(err)
	}
	if err := s.RetireWebOMEMO(ctx, u.Email, 42, "fixture-key"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(events.String(), `id="42"`) || !strings.Contains(events.String(), `id="7"`) {
		t.Fatal("incorrect retirement notification", events.String())
	}
	if _, err := s.SendWebOMEMO(ctx, u.Email, u.Email, retirementCipher); !errors.Is(err, ErrOMEMORetired) {
		t.Fatal("retired sender accepted", err)
	}
	session.dec = xml.NewDecoder(strings.NewReader(`<message to="` + u.Email + `" id="retired" type="chat">` + retirementCipher + `</message>`))
	start, _ := session.dec.Token()
	if err := session.handleMessage(start.(xml.StartElement)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "not-allowed") {
		t.Fatal("native sender accepted", output.String())
	}
	for node, raw := range map[string]string{listNode: retirementList, bundleNode: retirementBundle} {
		events.Reset()
		response := pepTestIQ(t, session, &output, "set", "", `<pubsub xmlns="http://jabber.org/protocol/pubsub"><publish node="`+node+`"><item id="current">`+raw+`</item></publish></pubsub>`)
		if node == bundleNode {
			if !strings.Contains(response, "forbidden") {
				t.Fatal(response)
			}
		} else {
			if !strings.Contains(response, "type='result'") || strings.Contains(events.String(), `id="42"`) || !strings.Contains(events.String(), `id="7"`) {
				t.Fatal(response, events.String())
			}
		}
	}
	// Neither reconnecting nor deleting/recreating the PEP node clears retirement.
	fresh := New(s.cfg, s.log, st, nil, nil)
	if err := fresh.deletePEPNode(ctx, u.Email, bundleNode); err != nil {
		t.Fatal(err)
	}
	if err := fresh.PublishWebOMEMO(ctx, u.Email, bundleNode, retirementBundle); !errors.Is(err, ErrOMEMORetired) {
		t.Fatal(err)
	}
	if err := fresh.PublishWebOMEMO(ctx, u.Email, legacyOMEMO+".bundles:00042", retirementBundle); !errors.Is(err, ErrOMEMORetired) {
		t.Fatal("numeric alias revived key", err)
	}
	items, err := fresh.ReadWebOMEMO(ctx, u.Email, bundleNode)
	if err != nil || len(items) != 0 {
		t.Fatal(items, err)
	}
	retired, err := fresh.RetiredWebOMEMO(ctx, u.Email)
	if err != nil || len(retired) != 1 || retired[0].ID != 42 || retired[0].At.IsZero() {
		t.Fatal(retired, err)
	}
	if err = fresh.RetireWebOMEMO(ctx, u.Email, 42, "fixture-key"); err != nil {
		t.Fatal("retry", err)
	}
	// Retirements do not affect other owners, the surviving device or non-OMEMO PEP.
	if err = fresh.PublishWebOMEMO(ctx, u.Email, legacyOMEMO+".bundles:7", retirementBundle); err != nil {
		t.Fatal(err)
	}
	if err = fresh.publishPEP(ctx, "other@example.com", bundleNode, "current", retirementBundle); err != nil {
		t.Fatal(err)
	}
	if err = fresh.publishPEP(ctx, u.Email, "urn:xmpp:avatar:metadata", "current", "<metadata/>"); err != nil {
		t.Fatal(err)
	}
}

func TestOMEMORetirementRollbackAndMissingBundle(t *testing.T) {
	s, _, u := testServer(t)
	ctx := context.Background()
	listNode := legacyOMEMO + ".devicelist"
	if err := s.PublishWebOMEMO(ctx, u.Email, listNode, retirementList); err != nil {
		t.Fatal(err)
	}
	// A lost device may never have published its bundle. Its advertised ID is enough.
	if _, err := s.db.Exec(`CREATE TRIGGER reject_retirement BEFORE UPDATE ON xmpp_pep_items BEGIN SELECT RAISE(ABORT,'fixture failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.RetireWebOMEMO(ctx, u.Email, 42, ""); err == nil {
		t.Fatal("injected failure ignored")
	}
	retired, err := s.RetiredWebOMEMO(ctx, u.Email)
	if err != nil || len(retired) != 0 {
		t.Fatal("retirement escaped rollback", retired, err)
	}
	items, err := s.ReadWebOMEMO(ctx, u.Email, listNode)
	if err != nil || len(items) != 1 || items[0].XML != retirementList {
		t.Fatal("list changed on failure", items, err)
	}
	if _, err = s.db.Exec(`DROP TRIGGER reject_retirement`); err != nil {
		t.Fatal(err)
	}
	if err = s.RetireWebOMEMO(ctx, u.Email, 42, ""); err != nil {
		t.Fatal(err)
	}
}

func TestOMEMORetirementConcurrentPublication(t *testing.T) {
	s, _, u := testServer(t)
	ctx := context.Background()
	node := legacyOMEMO + ".devicelist"
	if err := s.PublishWebOMEMO(ctx, u.Email, node, retirementList); err != nil {
		t.Fatal(err)
	}
	var database string
	var seq int
	var name string
	if err := s.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &database); err != nil {
		t.Fatal(err)
	}
	separate, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: database}})
	if err != nil {
		t.Fatal(err)
	}
	defer separate.Close()
	other := New(s.cfg, s.log, separate, nil, nil) // Separate connection AND mutex.
	var wg sync.WaitGroup
	start := make(chan struct{})
	failures := make(chan error, 3)
	wg.Add(3)
	for _, server := range []*Server{s, other} {
		go func(server *Server) {
			defer wg.Done()
			<-start
			for i := 0; i < 20; i++ {
				if err := server.PublishWebOMEMO(ctx, u.Email, node, retirementList); err != nil {
					failures <- err
					return
				}
			}
		}(server)
	}
	go func() { defer wg.Done(); <-start; failures <- s.RetireWebOMEMO(ctx, u.Email, 42, "") }()
	close(start)
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	items, err := s.ReadWebOMEMO(ctx, u.Email, node)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	var list omemoDeviceList
	if err = xml.Unmarshal([]byte(items[0].XML), &list); err != nil || len(list.Devices) != 1 || list.Devices[0].ID != 7 {
		t.Fatal(items, err)
	}
}
