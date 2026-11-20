package storage

import (
	"context"
	"database/sql"
	"time"
)

// SaveEWSContact applies a full vCard with an atomic content precondition.
// An explicit stable create ID makes a retry after a lost response idempotent.
func (s *Store) SaveEWSContact(ctx context.Context, userID string, o *AddressObject, expected string, create, remove bool) (*AddressObject, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var owner string
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT user_id FROM addressbooks WHERE id=?`), o.AddressBookID).Scan(&owner); err != nil || owner != userID {
		return nil, ErrUnauthorized
	}
	old, err := s.scanAddrObject(tx.QueryRowContext(ctx, s.rebind(`SELECT id,addressbook_id,uid,href_name,etag,data,size,updated_at FROM address_objects WHERE addressbook_id=? AND href_name=?`), o.AddressBookID, o.HrefName))
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	if create && err == nil {
		if old.ID != o.ID || old.UID != o.UID {
			return nil, ErrCalendarConflict
		}
		return old, nil
	}
	if !create && (err == ErrNotFound || old.ID != o.ID || old.ETag != expected) {
		return nil, ErrCalendarConflict
	}
	if create {
		var existingID string
		e := tx.QueryRowContext(ctx, s.rebind(`SELECT id FROM address_objects WHERE addressbook_id=? AND uid=?`), o.AddressBookID, o.UID).Scan(&existingID)
		if e == nil {
			return nil, ErrCalendarConflict
		}
		if e != sql.ErrNoRows {
			return nil, e
		}
	}
	now := time.Now().UTC()
	o.UpdatedAt, o.ETag, o.Size = now, ContentETag(o.Data), int64(len(o.Data))
	var result sql.Result
	switch {
	case remove:
		result, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM address_objects WHERE id=? AND etag=?`), o.ID, expected)
	case create:
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO address_objects (id,addressbook_id,uid,href_name,etag,data,size,updated_at) VALUES (?,?,?,?,?,?,?,?)`), o.ID, o.AddressBookID, o.UID, o.HrefName, o.ETag, o.Data, o.Size, now)
	default:
		result, err = tx.ExecContext(ctx, s.rebind(`UPDATE address_objects SET data=?,etag=?,size=?,updated_at=? WHERE id=? AND etag=?`), o.Data, o.ETag, o.Size, now, o.ID, expected)
	}
	if err != nil {
		return nil, err
	}
	if result != nil {
		n, e := result.RowsAffected()
		if e != nil {
			return nil, e
		}
		if n != 1 {
			return nil, ErrCalendarConflict
		}
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE addressbooks SET ctag=? WHERE id=?`), newCTag(), o.AddressBookID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return o, nil
}
