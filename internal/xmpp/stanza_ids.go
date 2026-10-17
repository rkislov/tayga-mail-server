package xmpp

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

const stanzaIDNamespace = "urn:xmpp:sid:0"

// stampMessage replaces only IDs asserted on behalf of the supplied accounts.
// It preserves origin-id, third-party IDs and encrypted payloads. Explicit client
// namespaces also keep archived messages valid inside MAM/Carbons envelopes.
func stampMessage(raw string, ids map[string]string) (string, error) {
	dec := xml.NewDecoder(strings.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return "", err
	}
	root, ok := tok.(xml.StartElement)
	if !ok || root.Name.Local != "message" || (root.Name.Space != "" && root.Name.Space != "jabber:client" && root.Name.Space != "jabber:component:accept") {
		return "", fmt.Errorf("expected message stanza")
	}
	root.Name.Space = "jabber:client"
	var out strings.Builder
	writeStart(&out, root)
	for {
		tok, err = dec.Token()
		if err != nil {
			return "", err
		}
		switch child := tok.(type) {
		case xml.StartElement:
			if child.Name == (xml.Name{Space: stanzaIDNamespace, Local: "stanza-id"}) {
				by, e := NormalizeJID(attr(child, "by"))
				if _, replace := ids[by.Full()]; e == nil && replace {
					if err := dec.Skip(); err != nil {
						return "", err
					}
					continue
				}
			}
			content, err := readInnerXML(dec, child)
			if err != nil {
				return "", err
			}
			if child.Name.Space == "" || child.Name.Space == "jabber:component:accept" {
				child.Name.Space = "jabber:client"
			}
			writeStart(&out, child)
			out.WriteString(content)
			out.WriteString("</" + child.Name.Local + ">")
		case xml.CharData:
			out.WriteString(xmlEscape(string(child)))
		case xml.EndElement:
			// Sorting also makes serialization deterministic for legacy archive rows.
			owners := make([]string, 0, len(ids))
			for by := range ids {
				owners = append(owners, by)
			}
			sort.Strings(owners)
			for _, by := range owners {
				if id := ids[by]; id != "" {
					fmt.Fprintf(&out, `<stanza-id xmlns='%s' by='%s' id='%s'/>`, stanzaIDNamespace, xmlEscape(by), xmlEscape(id))
				}
			}
			out.WriteString("</message>")
			return out.String(), nil
		}
	}
}

// prepareArchivedMessage runs once before any delivery, offline storage or carbon
// copies. A client message ID is never used as an archive key: two devices may
// legitimately reuse it. Both local archives commit together before delivery.
func (s *Server) prepareArchivedMessage(ctx context.Context, raw string) (string, string, error) {
	from, to := parseMessageAddrs(raw)
	from, to = ParseJID(from).Bare(), ParseJID(to).Bare()
	id := storage.NewID()
	ids := map[string]string{}
	archive := map[string]string{}
	if s.db != nil && s.store != nil {
		for _, pair := range [][2]string{{from, to}, {to, from}} {
			owner, peer := pair[0], pair[1]
			if _, seen := ids[owner]; seen {
				continue
			}
			u, err := s.store.GetUserByEmail(ctx, owner)
			if errors.Is(err, storage.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
				continue // Components/remote peers do not own local account archives.
			}
			if err != nil {
				return "", "", err
			}
			if u == nil {
				continue
			}
			ids[owner] = id
			allowed, err := s.shouldArchive(ctx, owner, peer, raw)
			if err != nil {
				return "", "", err
			}
			if allowed {
				archive[owner] = peer
			}
		}
	}
	stanza, err := stampMessage(raw, ids)
	if err != nil || len(archive) == 0 {
		return stanza, id, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	for owner, peer := range archive {
		if _, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_mam
			(id, owner_bare_jid, with_bare_jid, stanza_id, stanza, created_at) VALUES (?, ?, ?, ?, ?, ?)`),
			storage.NewID(), owner, peer, id, stanza, now); err != nil {
			return "", "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return stanza, id, nil
}

// The SSE ID must match ChatHistory/MAM for this owner, not the sender's
// independently chosen message id. Only inspect direct children of the stanza.
func stanzaEventID(raw, owner string) string {
	var msg struct {
		ID  string `xml:"id,attr"`
		IDs []struct {
			By string `xml:"by,attr"`
			ID string `xml:"id,attr"`
		} `xml:"urn:xmpp:sid:0 stanza-id"`
	}
	if xml.Unmarshal([]byte(raw), &msg) != nil {
		return ""
	}
	for _, id := range msg.IDs {
		if id.By == owner && id.ID != "" {
			return id.ID
		}
	}
	return msg.ID
}
