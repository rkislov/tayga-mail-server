package xmpp

import (
	"context"
	"database/sql"
	"time"

	"github.com/tayga/tms/internal/storage"
)

type rosterItem struct {
	JID          string
	Name         string
	Subscription string
	GroupsJSON   string
	Ask          string
}

type mamRow struct {
	StanzaID  string
	Stanza    string
	CreatedAt time.Time
}

type pepItem struct {
	ItemID  string
	Payload string
}

func (s *Server) listRoster(ctx context.Context, userID string) ([]rosterItem, error) {
	if s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(
		`SELECT jid, name, subscription, groups, ask FROM xmpp_roster WHERE user_id = ? ORDER BY jid`),
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []rosterItem
	for rows.Next() {
		var it rosterItem
		if err := rows.Scan(&it.JID, &it.Name, &it.Subscription, &it.GroupsJSON, &it.Ask); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Server) upsertRoster(ctx context.Context, userID string, it rosterItem) error {
	if s.db == nil {
		return nil
	}
	if it.Subscription == "" {
		it.Subscription = "none"
	}
	if it.GroupsJSON == "" {
		it.GroupsJSON = "[]"
	}
	id := storage.NewID()
	_, err := s.db.ExecContext(ctx, s.rebind(`
		INSERT INTO xmpp_roster (id, user_id, jid, name, subscription, groups, ask, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, jid) DO UPDATE SET
			name = excluded.name,
			subscription = excluded.subscription,
			groups = excluded.groups,
			ask = excluded.ask,
			updated_at = excluded.updated_at`),
		id, userID, it.JID, it.Name, it.Subscription, it.GroupsJSON, it.Ask, time.Now().UTC(),
	)
	return err
}

func (s *Server) deleteRoster(ctx context.Context, userID, jid string) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, s.rebind(
		`DELETE FROM xmpp_roster WHERE user_id = ? AND jid = ?`), userID, jid)
	return err
}

func (s *Server) flushOffline(ctx context.Context, userID string, send func([]byte)) error {
	if s.db == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(
		`SELECT id, stanza FROM xmpp_offline WHERE user_id = ? ORDER BY created_at`), userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id, stanza string
		if err := rows.Scan(&id, &stanza); err != nil {
			return err
		}
		send([]byte(stanza))
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		_, _ = s.db.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_offline WHERE id = ?`), id)
	}
	return nil
}

func (s *Server) ensurePEPNode(ctx context.Context, ownerBare, node string) (string, error) {
	if s.db == nil {
		return "", sql.ErrConnDone
	}
	var id string
	err := s.db.QueryRowContext(ctx, s.rebind(
		`SELECT id FROM xmpp_pep_nodes WHERE owner_bare_jid = ? AND node = ?`),
		ownerBare, node,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	id = storage.NewID()
	_, err = s.db.ExecContext(ctx, s.rebind(
		`INSERT INTO xmpp_pep_nodes (id, owner_bare_jid, node, access_model, updated_at) VALUES (?, ?, ?, 'presence', ?)`),
		id, ownerBare, node, time.Now().UTC(),
	)
	return id, err
}

func (s *Server) publishPEP(ctx context.Context, ownerBare, node, itemID, payload string) error {
	nodeID, err := s.ensurePEPNode(ctx, ownerBare, node)
	if err != nil {
		return err
	}
	if itemID == "" {
		itemID = "current"
	}
	id := storage.NewID()
	_, err = s.db.ExecContext(ctx, s.rebind(`
		INSERT INTO xmpp_pep_items (id, node_id, item_id, payload, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(node_id, item_id) DO UPDATE SET
			payload = excluded.payload,
			updated_at = excluded.updated_at`),
		id, nodeID, itemID, payload, time.Now().UTC(),
	)
	return err
}

func (s *Server) getPEPItems(ctx context.Context, ownerBare, node string) ([]pepItem, error) {
	if s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(`
		SELECT i.item_id, i.payload
		FROM xmpp_pep_items i
		JOIN xmpp_pep_nodes n ON n.id = i.node_id
		WHERE n.owner_bare_jid = ? AND n.node = ?
		ORDER BY i.item_id`), ownerBare, node)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pepItem
	for rows.Next() {
		var it pepItem
		if err := rows.Scan(&it.ItemID, &it.Payload); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Server) deletePEPNode(ctx context.Context, ownerBare, node string) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, s.rebind(
		`DELETE FROM xmpp_pep_nodes WHERE owner_bare_jid = ? AND node = ?`), ownerBare, node)
	return err
}

func (s *Server) queryMAM(ctx context.Context, ownerBare, withBare string, limit int) ([]mamRow, error) {
	if s.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	if withBare != "" {
		rows, err = s.db.QueryContext(ctx, s.rebind(`
			SELECT stanza_id, stanza, created_at FROM xmpp_mam
			WHERE owner_bare_jid = ? AND with_bare_jid = ?
			ORDER BY created_at DESC LIMIT ?`), ownerBare, withBare, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, s.rebind(`
			SELECT stanza_id, stanza, created_at FROM xmpp_mam
			WHERE owner_bare_jid = ?
			ORDER BY created_at DESC LIMIT ?`), ownerBare, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mamRow
	for rows.Next() {
		var r mamRow
		if err := rows.Scan(&r.StanzaID, &r.Stanza, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	// reverse to chronological
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}
