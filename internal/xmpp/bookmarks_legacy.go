package xmpp

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

const legacyBookmarksNode = "storage:bookmarks"

var errInvalidBookmarks = errors.New("invalid bookmarks")

// Keep resolved namespaces, including unknown extension elements/attributes.
// A bounded token representation also rejects trailing documents and deep XML.
func bookmarkTokens(raw string, root xml.Name) ([]xml.Token, error) {
	if len(raw) > 256*1024 {
		return nil, errInvalidBookmarks
	}
	d := xml.NewDecoder(strings.NewReader(raw))
	var tokens []xml.Token
	depth, done := 0, false
	for {
		token, err := d.Token()
		if err == io.EOF && done {
			return tokens, nil
		}
		if err != nil {
			return nil, errInvalidBookmarks
		}
		switch t := token.(type) {
		case xml.StartElement:
			if done || (depth == 0 && t.Name != root) || depth >= 32 {
				return nil, errInvalidBookmarks
			}
			depth++
		case xml.EndElement:
			depth--
			done = depth == 0
		case xml.CharData:
			if depth == 0 {
				if strings.TrimSpace(string(t)) != "" {
					return nil, errInvalidBookmarks
				}
				continue
			}
		case xml.Comment:
			continue
		default:
			return nil, errInvalidBookmarks
		}
		tokens = append(tokens, xml.CopyToken(token))
	}
}

func encodeBookmarkTokens(tokens []xml.Token) string {
	var b strings.Builder
	e := xml.NewEncoder(&b)
	for _, token := range tokens {
		if start, ok := token.(xml.StartElement); ok {
			attrs := make([]xml.Attr, 0, len(start.Attr))
			for _, a := range start.Attr {
				if a.Name.Space != "xmlns" && a.Name != (xml.Name{Local: "xmlns"}) {
					attrs = append(attrs, a)
				}
			}
			start.Attr = attrs
			token = start
		}
		_ = e.EncodeToken(token) // Tokens came from the validating decoder.
	}
	_ = e.Flush()
	return b.String()
}

func bookmarkChildren(tokens []xml.Token) [][]xml.Token {
	var children [][]xml.Token
	depth, start := 0, 0
	for i, token := range tokens[1 : len(tokens)-1] {
		switch token.(type) {
		case xml.StartElement:
			if depth == 0 {
				start = i + 1
			}
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 {
				children = append(children, tokens[start:i+2])
			}
		}
	}
	return children
}

func convertBookmark(tokens []xml.Token, from, to, jid string) []xml.Token {
	out := make([]xml.Token, 0, len(tokens))
	for i, token := range tokens {
		token = xml.CopyToken(token)
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Space == from {
				t.Name.Space = to
			}
			if i == 0 {
				attrs := make([]xml.Attr, 0, len(t.Attr)+1)
				for _, a := range t.Attr {
					if a.Name != (xml.Name{Local: "jid"}) {
						attrs = append(attrs, a)
					}
				}
				if jid != "" {
					attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "jid"}, Value: jid})
				}
				t.Attr = attrs
			}
			token = t
		case xml.EndElement:
			if t.Name.Space == from {
				t.Name.Space = to
			}
			token = t
		}
		out = append(out, token)
	}
	return out
}

// The old single-item representation is a projection, not a second source of
// conference state. Its stored payload keeps URL/other legacy-only bookmarks.
func (s *Server) legacyBookmarkItem(ctx context.Context, owner string) (pepItem, error) {
	item := pepItem{ItemID: "current"}
	// Read both representations in one database snapshot, including on PostgreSQL.
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT n.node,i.item_id,i.payload FROM xmpp_pep_nodes n JOIN xmpp_pep_items i ON i.node_id=n.id WHERE n.owner_bare_jid=? AND n.node IN (?,?) ORDER BY n.node,i.item_id`), owner, legacyBookmarksNode, bookmarksNode)
	if err != nil {
		return item, err
	}
	var old, items []pepItem
	for rows.Next() {
		var node string
		var it pepItem
		if err = rows.Scan(&node, &it.ItemID, &it.Payload); err != nil {
			rows.Close()
			return item, err
		}
		if node == legacyBookmarksNode {
			old = append(old, it)
		} else {
			items = append(items, it)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return item, err
	}
	root := xml.StartElement{Name: xml.Name{Space: legacyBookmarksNode, Local: "storage"}}
	tokens := []xml.Token{root}
	if len(old) > 0 {
		item.ItemID = old[0].ItemID
		prior, err := bookmarkTokens(old[0].Payload, root.Name)
		if err != nil {
			return item, err
		}
		for _, child := range bookmarkChildren(prior) {
			if child[0].(xml.StartElement).Name != (xml.Name{Space: legacyBookmarksNode, Local: "conference"}) {
				tokens = append(tokens, child...)
			}
		}
	}
	for _, it := range items {
		conference, err := bookmarkTokens(it.Payload, xml.Name{Space: bookmarksNode, Local: "conference"})
		if err != nil {
			return item, err
		}
		tokens = append(tokens, convertBookmark(conference, bookmarksNode, legacyBookmarksNode, it.ItemID)...)
	}
	tokens = append(tokens, root.End())
	item.Payload = encodeBookmarkTokens(tokens)
	return item, nil
}

func parseLegacyBookmarks(raw string) (map[string]string, string, error) {
	root := xml.StartElement{Name: xml.Name{Space: legacyBookmarksNode, Local: "storage"}}
	tokens, err := bookmarkTokens(raw, root.Name)
	if err != nil {
		return nil, "", err
	}
	items := map[string]string{}
	extras := []xml.Token{root}
	for _, child := range bookmarkChildren(tokens) {
		start := child[0].(xml.StartElement)
		if start.Name != (xml.Name{Space: legacyBookmarksNode, Local: "conference"}) {
			extras = append(extras, child...)
			continue
		}
		jid, err := NormalizeJID(attr(start, "jid"))
		if err != nil || jid.Local == "" || jid.Resource != "" {
			return nil, "", errInvalidBookmarks
		}
		if _, exists := items[jid.Bare()]; exists {
			return nil, "", errInvalidBookmarks
		}
		payload := encodeBookmarkTokens(convertBookmark(child, legacyBookmarksNode, bookmarksNode, ""))
		if !validBookmark(jid.Bare(), payload) {
			return nil, "", errInvalidBookmarks
		}
		items[jid.Bare()] = payload
	}
	extras = append(extras, root.End())
	return items, encodeBookmarkTokens(extras), nil
}

func preserveBookmarkExtensions(next, previous string) string {
	root := xml.Name{Space: bookmarksNode, Local: "conference"}
	n, err := bookmarkTokens(next, root)
	if err != nil {
		return next
	}
	for _, child := range bookmarkChildren(n) {
		if child[0].(xml.StartElement).Name == (xml.Name{Space: bookmarksNode, Local: "extensions"}) {
			return next
		}
	}
	p, err := bookmarkTokens(previous, root)
	if err == nil {
		for _, child := range bookmarkChildren(p) {
			if child[0].(xml.StartElement).Name == (xml.Name{Space: bookmarksNode, Local: "extensions"}) {
				n = append(append(n[:len(n)-1], child...), xml.EndElement{Name: root})
				break
			}
		}
	}
	return encodeBookmarkTokens(n)
}

// Import a pre-upgrade legacy PEP item once, without overwriting newer native
// bookmarks. Stripping conference children makes later native deletions final.
func (s *Server) importLegacyBookmarksTx(ctx context.Context, tx *sql.Tx, nodeID, legacyID string) error {
	var id, raw string
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT item_id,payload FROM xmpp_pep_items WHERE node_id=? ORDER BY updated_at DESC LIMIT 1`), legacyID).Scan(&id, &raw)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	items, extras, err := parseLegacyBookmarks(raw)
	if err != nil || len(items) == 0 {
		return err
	}
	for jid, payload := range items {
		if _, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_pep_items(id,node_id,item_id,payload,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(node_id,item_id) DO NOTHING`), storage.NewID(), nodeID, jid, payload, time.Now().UTC()); err != nil {
			return err
		}
	}
	// Old generic nodes could contain several items; XEP-0048 has one document.
	if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_pep_items WHERE node_id=?`), legacyID); err != nil {
		return err
	}
	return (&Session{srv: s}).upsertBookmarkTx(ctx, tx, legacyID, id, extras)
}

func (s *Session) replaceLegacyBookmarks(ctx context.Context, owner, itemID, raw string) error {
	items, extras, err := parseLegacyBookmarks(raw)
	if err != nil {
		return err
	}
	unlock := s.srv.lockPEPPublication(owner)
	defer unlock()
	nodeID, err := s.srv.ensurePEPNode(ctx, owner, bookmarksNode)
	if err != nil {
		return err
	}
	legacyID, err := s.srv.ensurePEPNode(ctx, owner, legacyBookmarksNode)
	if err != nil {
		return err
	}
	tx, err := s.srv.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, s.srv.rebind(`SELECT item_id,payload FROM xmpp_pep_items WHERE node_id=?`), nodeID)
	if err != nil {
		return err
	}
	previous := map[string]string{}
	for rows.Next() {
		var id, payload string
		if err = rows.Scan(&id, &payload); err != nil {
			rows.Close()
			return err
		}
		previous[id] = payload
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var events []string
	for id, payload := range items {
		payload = preserveBookmarkExtensions(payload, previous[id])
		prior, _ := bookmarkTokens(previous[id], xml.Name{Space: bookmarksNode, Local: "conference"})
		if payload == encodeBookmarkTokens(prior) {
			continue
		}
		if err = s.upsertBookmarkTx(ctx, tx, nodeID, id, payload); err != nil {
			return err
		}
		events = append(events, `<item id='`+xmlEscape(id)+`'>`+payload+`</item>`)
	}
	for id := range previous {
		if _, kept := items[id]; kept {
			continue
		}
		if _, err = tx.ExecContext(ctx, s.srv.rebind(`DELETE FROM xmpp_pep_items WHERE node_id=? AND item_id=?`), nodeID, id); err != nil {
			return err
		}
		events = append(events, `<retract id='`+xmlEscape(id)+`'/>`)
	}
	if _, err = tx.ExecContext(ctx, s.srv.rebind(`DELETE FROM xmpp_pep_items WHERE node_id=?`), legacyID); err != nil {
		return err
	}
	if err = s.upsertBookmarkTx(ctx, tx, legacyID, itemID, extras); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for _, event := range events {
		s.notifyPEPOnly(ctx, owner, bookmarksNode, event)
	}
	s.notifyLegacyBookmarks(ctx, owner)
	return nil
}

func (s *Session) upsertBookmarkTx(ctx context.Context, tx *sql.Tx, nodeID, id, payload string) error {
	_, err := tx.ExecContext(ctx, s.srv.rebind(`INSERT INTO xmpp_pep_items(id,node_id,item_id,payload,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(node_id,item_id) DO UPDATE SET payload=excluded.payload,updated_at=excluded.updated_at`), storage.NewID(), nodeID, id, payload, time.Now().UTC())
	return err
}

func (s *Session) notifyLegacyBookmarks(ctx context.Context, owner string) {
	item, err := s.srv.legacyBookmarkItem(ctx, owner)
	if err != nil {
		s.srv.logBookmarkError(err)
		return
	}
	s.notifyPEPOnly(ctx, owner, legacyBookmarksNode, `<item id='`+xmlEscape(item.ItemID)+`'>`+item.Payload+`</item>`)
}
