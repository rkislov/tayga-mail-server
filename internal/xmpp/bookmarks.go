package xmpp

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

const bookmarksNode = "urn:xmpp:bookmarks:1"

func validBookmark(id, payload string) bool {
	jid, err := NormalizeJID(id)
	if err != nil || jid.Local == "" || jid.Resource != "" || jid.Bare() != id {
		return false
	}
	var c struct {
		XMLName  xml.Name
		Autojoin string `xml:"autojoin,attr"`
	}
	decoder := xml.NewDecoder(strings.NewReader(payload))
	if decoder.Decode(&c) != nil || c.XMLName != (xml.Name{Space: bookmarksNode, Local: "conference"}) {
		return false
	}
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return false
		}
		text, ok := token.(xml.CharData)
		if !ok || strings.TrimSpace(string(text)) != "" {
			return false
		}
	}
	return c.Autojoin == "" || c.Autojoin == "true" || c.Autojoin == "false" || c.Autojoin == "1" || c.Autojoin == "0"
}

// Seed each membership once. A client may subsequently disable autojoin, change
// its nickname/extensions, or remove the bookmark without it being resurrected.
func (s *Server) syncRoomBookmarks(ctx context.Context, owner string) error {
	s.mucMu.Lock()
	defer s.mucMu.Unlock()
	return s.syncRoomBookmarksLocked(ctx, owner)
}

// Caller holds mucMu, serializing room membership changes with the snapshot.
func (s *Server) syncRoomBookmarksLocked(ctx context.Context, owner string) error {
	unlock := s.lockPEPPublication(owner)
	defer unlock()
	nodeID, err := s.ensurePEPNode(ctx, owner, bookmarksNode)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Repair old generic PEP nodes as well: bookmarks are always private and
	// multi-item. Never publish room names/nicknames to roster contacts.
	if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE xmpp_pep_nodes SET access_model='whitelist',max_items=0,send_last=?,notify_retract=?,notify_delete=? WHERE id=?`), true, true, true, nodeID); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT r.jid,r.name FROM web_chat_rooms r JOIN web_chat_members m ON m.room_jid=r.jid JOIN users u ON u.email=m.email JOIN users o ON o.email=r.owner_email WHERE m.email=? AND m.bookmark_seeded=? AND u.enabled=? AND o.enabled=? AND u.tenant_id=o.tenant_id AND u.auth_source<>'resource'`), owner, false, true, true)
	if err != nil {
		return err
	}
	var rooms []ChatRoom
	for rows.Next() {
		var r ChatRoom
		if err = rows.Scan(&r.JID, &r.Name); err != nil {
			rows.Close()
			return err
		}
		rooms = append(rooms, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var added []pepItem
	for _, r := range rooms {
		// Native rooms are not visible until their creating client submits config.
		if s.mucPending[r.JID] != nil {
			continue
		}
		payload := fmt.Sprintf(`<conference xmlns='%s' name='%s' autojoin='true'><nick>%s</nick></conference>`, bookmarksNode, xmlEscape(r.Name), xmlEscape(ParseJID(owner).Local))
		result, e := tx.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_pep_items(id,node_id,item_id,payload,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(node_id,item_id) DO NOTHING`), storage.NewID(), nodeID, r.JID, payload, time.Now().UTC())
		if e != nil {
			return e
		}
		if n, _ := result.RowsAffected(); n > 0 {
			added = append(added, pepItem{ItemID: r.JID, Payload: payload})
		}
		if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE web_chat_members SET bookmark_seeded=? WHERE room_jid=? AND email=?`), true, r.JID, owner); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	publisher := &Session{srv: s}
	for _, it := range added {
		publisher.notifyPEP(ctx, owner, bookmarksNode, `<item id='`+xmlEscape(it.ItemID)+`'>`+it.Payload+`</item>`)
	}
	return nil
}

// Removing membership also removes the managed bookmark from all own devices.
func (s *Server) removeRoomBookmark(ctx context.Context, owner, room string) error {
	unlock := s.lockPEPPublication(owner)
	defer unlock()
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_pep_items WHERE item_id=? AND node_id IN (SELECT id FROM xmpp_pep_nodes WHERE owner_bare_jid=? AND node=?)`), room, owner, bookmarksNode)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n > 0 {
		(&Session{srv: s}).notifyPEP(ctx, owner, bookmarksNode, `<retract id='`+xmlEscape(room)+`'/>`)
	}
	return nil
}

func (s *Server) logBookmarkError(err error) {
	if err != nil && s.log != nil {
		s.log.Error("xmpp room bookmark synchronization failed", "error", strings.TrimSpace(err.Error()))
	}
}
