package xmpp

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrOMEMORetired = errors.New("OMEMO key retired / Ключ OMEMO отозван")
var ErrOMEMOKeyChanged = errors.New("OMEMO key changed; refresh devices / Ключ изменился; обновите устройства")

// Do not hold a global lock while notifying a slow resource. References include
// waiting publishers so an owner's lock cannot be replaced while it is in use.
type pepOwnerLock struct {
	mu   sync.Mutex
	refs int
}

func (s *Server) lockPEPPublication(owner string) func() {
	s.pepPublishMu.Lock()
	if s.pepOwners == nil {
		s.pepOwners = map[string]*pepOwnerLock{}
	}
	lock := s.pepOwners[owner]
	if lock == nil {
		lock = &pepOwnerLock{}
		s.pepOwners[owner] = lock
	}
	lock.refs++
	s.pepPublishMu.Unlock()
	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		s.pepPublishMu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(s.pepOwners, owner)
		}
		s.pepPublishMu.Unlock()
	}
}

type RetiredOMEMODevice struct {
	ID uint32    `json:"id"`
	At time.Time `json:"retired_at"`
}

type omemoDeviceList struct {
	XMLName xml.Name `xml:"eu.siacs.conversations.axolotl list"`
	Devices []struct {
		ID    uint32 `xml:"id,attr"`
		Label string `xml:"label,attr,omitempty"`
	} `xml:"eu.siacs.conversations.axolotl device"`
}

func legacyBundleID(node string) uint32 {
	if !strings.HasPrefix(node, legacyOMEMO+".bundles:") {
		return 0
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(node, legacyOMEMO+".bundles:"), 10, 31)
	if err != nil {
		return 0
	}
	return uint32(id)
}

// The first write takes a database lock before reading PEP state. A no-op UPDATE
// serializes publishers and retirement across connections on SQLite/PostgreSQL.
func (s *Server) lockOMEMOOwner(ctx context.Context, tx *sql.Tx, owner string) error {
	if _, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_omemo_owners(owner) VALUES (?) ON CONFLICT(owner) DO NOTHING`), owner); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, s.rebind(`UPDATE xmpp_omemo_owners SET owner=owner WHERE owner=?`), owner)
	return err
}

func (s *Server) filterOMEMOPublication(ctx context.Context, tx *sql.Tx, owner, node, payload string) (string, error) {
	if id := legacyBundleID(node); id != 0 {
		var count int
		if err := tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM xmpp_omemo_retired WHERE owner=? AND device_id=?`), owner, id).Scan(&count); err != nil {
			return "", err
		}
		if count != 0 {
			return "", ErrOMEMORetired
		}
		return payload, nil
	}
	var list omemoDeviceList
	if len(payload) > 128*1024 || xml.Unmarshal([]byte(payload), &list) != nil || len(list.Devices) > 100 {
		return "", fmt.Errorf("invalid OMEMO device list")
	}
	for _, device := range list.Devices {
		if device.ID == 0 || device.ID > 2147483647 {
			return "", fmt.Errorf("invalid OMEMO device ID")
		}
	}
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT device_id FROM xmpp_omemo_retired WHERE owner=?`), owner)
	if err != nil {
		return "", err
	}
	retired := map[uint32]bool{}
	for rows.Next() {
		var id uint32
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return "", err
		}
		retired[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", err
	}
	kept := list.Devices[:0]
	removed := false
	for _, device := range list.Devices {
		if retired[device.ID] {
			removed = true
		} else {
			kept = append(kept, device)
		}
	}
	if !removed {
		return payload, nil
	}
	list.Devices = kept
	raw, err := xml.Marshal(list)
	return string(raw), err
}

// Only the authenticated owner is supplied by the HTTP handler, never a caller's
// owner parameter. Retirement is an account-wide PEP policy, not credential revocation.
func (s *Server) RetireWebOMEMO(ctx context.Context, owner string, id uint32, expectedIdentity string) error {
	if id == 0 || id > 2147483647 {
		return fmt.Errorf("invalid OMEMO device ID")
	}
	owner = ParseJID(owner).Bare()
	user, err := s.store.GetUserByEmail(ctx, owner)
	if err != nil || !user.Enabled {
		return fmt.Errorf("owner unavailable")
	}
	unlock := s.lockPEPPublication(owner)
	defer unlock()
	listNode := legacyOMEMO + ".devicelist"
	if _, err = s.ensurePEPNode(ctx, owner, listNode); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.lockOMEMOOwner(ctx, tx, owner); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM xmpp_omemo_retired WHERE owner=? AND device_id=?`), owner, id).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return nil
	} // Idempotent, never restores an old key.
	var bundle string
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT i.payload FROM xmpp_pep_items i JOIN xmpp_pep_nodes n ON n.id=i.node_id WHERE n.owner_bare_jid=? AND n.node=? ORDER BY i.updated_at DESC LIMIT 1`), owner, legacyOMEMO+".bundles:"+strconv.FormatUint(uint64(id), 10)).Scan(&bundle)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var key struct {
		Identity string `xml:"eu.siacs.conversations.axolotl identityKey"`
	}
	if bundle != "" && xml.Unmarshal([]byte(bundle), &key) != nil {
		return fmt.Errorf("invalid device bundle")
	}
	if strings.TrimSpace(key.Identity) != strings.TrimSpace(expectedIdentity) {
		return ErrOMEMOKeyChanged
	}
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT i.item_id,i.payload FROM xmpp_pep_items i JOIN xmpp_pep_nodes n ON n.id=i.node_id WHERE n.owner_bare_jid=? AND n.node=?`), owner, listNode)
	if err != nil {
		return err
	}
	items := []pepItem{}
	found := bundle != ""
	for rows.Next() {
		var item pepItem
		if err = rows.Scan(&item.ItemID, &item.Payload); err != nil {
			rows.Close()
			return err
		}
		var list omemoDeviceList
		if err = xml.Unmarshal([]byte(item.Payload), &list); err != nil {
			rows.Close()
			return err
		}
		for _, device := range list.Devices {
			if device.ID == id {
				found = true
			}
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !found {
		return sql.ErrNoRows
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_omemo_retired(owner,device_id,retired_at) VALUES (?,?,?)`), owner, id, time.Now().UTC()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_pep_items WHERE node_id IN (SELECT id FROM xmpp_pep_nodes WHERE owner_bare_jid=? AND node=?)`), owner, legacyOMEMO+".bundles:"+strconv.FormatUint(uint64(id), 10)); err != nil {
		return err
	}
	for i := range items {
		items[i].Payload, err = s.filterOMEMOPublication(ctx, tx, owner, listNode, items[i].Payload)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE xmpp_pep_items SET payload=?,updated_at=? WHERE item_id=? AND node_id IN (SELECT id FROM xmpp_pep_nodes WHERE owner_bare_jid=? AND node=?)`), items[i].Payload, time.Now().UTC(), items[i].ItemID, owner, listNode); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	session := &Session{srv: s, user: user, JID: ParseJID(owner + "/web")}
	for _, item := range items {
		session.notifyPEP(ctx, owner, listNode, `<item id="`+xmlEscape(item.ItemID)+`">`+item.Payload+`</item>`)
	}
	return nil
}

func (s *Server) RetiredWebOMEMO(ctx context.Context, owner string) ([]RetiredOMEMODevice, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT device_id,retired_at FROM xmpp_omemo_retired WHERE owner=? ORDER BY retired_at,device_id`), ParseJID(owner).Bare())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RetiredOMEMODevice{}
	for rows.Next() {
		var d RetiredOMEMODevice
		if err = rows.Scan(&d.ID, &d.At); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Server) checkOMEMODevice(ctx context.Context, owner string, id uint32) error {
	var count int
	if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM xmpp_omemo_retired WHERE owner=? AND device_id=?`), ParseJID(owner).Bare(), id).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return ErrOMEMORetired
	}
	return nil
}

func (s *Server) checkOMEMOSender(ctx context.Context, owner, raw string) error {
	var wire encryptedWire
	if xml.Unmarshal([]byte(raw), &wire) != nil {
		return nil
	} // Other encryption namespaces are unaffected.
	return s.checkOMEMODevice(ctx, owner, wire.Header.SID)
}
