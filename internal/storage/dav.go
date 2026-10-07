package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Calendar is a CalDAV collection owned by a user.
type Calendar struct {
	ID          string
	UserID      string
	Name        string
	DisplayName string
	Description string
	CTag        string
	CreatedAt   time.Time
}

// CalendarObject is an iCalendar resource inside a calendar.
type CalendarObject struct {
	ID         string
	CalendarID string
	UID        string
	HrefName   string
	ETag       string
	Data       string
	Size       int64
	Component  string
	DTStart    *time.Time
	DTEnd      *time.Time
	UpdatedAt  time.Time
}

// AddressBook is a CardDAV collection owned by a user.
type AddressBook struct {
	ID          string
	UserID      string
	Name        string
	DisplayName string
	Description string
	CTag        string
	CreatedAt   time.Time
}

// AddressObject is a vCard resource inside an address book.
type AddressObject struct {
	ID            string
	AddressBookID string
	UID           string
	HrefName      string
	ETag          string
	Data          string
	Size          int64
	UpdatedAt     time.Time
}

func newCTag() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ContentETag returns a stable etag for opaque content.
func ContentETag(data string) string {
	sum := sha256.Sum256([]byte(data))
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}

func (s *Store) EnsureCalendar(ctx context.Context, userID, name, displayName string) (*Calendar, error) {
	cal, err := s.GetCalendarByName(ctx, userID, name)
	if err == nil {
		return cal, nil
	}
	if err != ErrNotFound {
		return nil, err
	}
	if displayName == "" {
		displayName = name
	}
	return s.CreateCalendar(ctx, &Calendar{
		UserID:      userID,
		Name:        name,
		DisplayName: displayName,
		CTag:        newCTag(),
	})
}

func (s *Store) CreateCalendar(ctx context.Context, c *Calendar) (*Calendar, error) {
	now := time.Now().UTC()
	c.CreatedAt = now
	if c.ID == "" {
		c.ID = NewID()
	}
	if c.CTag == "" {
		c.CTag = newCTag()
	}
	q := s.rebind(`INSERT INTO calendars (id, user_id, name, display_name, description, ctag, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, q, c.ID, c.UserID, c.Name, c.DisplayName, c.Description, c.CTag, now)
	return c, err
}

func (s *Store) ListCalendars(ctx context.Context, userID string) ([]*Calendar, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, description, ctag, created_at FROM calendars WHERE user_id = ? ORDER BY name`)
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Calendar
	for rows.Next() {
		c := &Calendar{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.DisplayName, &c.Description, &c.CTag, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCalendarByName(ctx context.Context, userID, name string) (*Calendar, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, description, ctag, created_at FROM calendars WHERE user_id = ? AND name = ?`)
	c := &Calendar{}
	err := s.db.QueryRowContext(ctx, q, userID, name).Scan(&c.ID, &c.UserID, &c.Name, &c.DisplayName, &c.Description, &c.CTag, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) GetCalendarByID(ctx context.Context, userID, id string) (*Calendar, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, description, ctag, created_at FROM calendars WHERE user_id = ? AND id = ?`)
	c := &Calendar{}
	err := s.db.QueryRowContext(ctx, q, userID, id).Scan(&c.ID, &c.UserID, &c.Name, &c.DisplayName, &c.Description, &c.CTag, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) DeleteCalendar(ctx context.Context, userID, name string) error {
	q := s.rebind(`DELETE FROM calendars WHERE user_id = ? AND name = ?`)
	res, err := s.db.ExecContext(ctx, q, userID, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) bumpCalendarCTag(ctx context.Context, calendarID string) error {
	q := s.rebind(`UPDATE calendars SET ctag = ? WHERE id = ?`)
	_, err := s.db.ExecContext(ctx, q, newCTag(), calendarID)
	return err
}

func (s *Store) ListCalendarObjects(ctx context.Context, calendarID string) ([]*CalendarObject, error) {
	q := s.rebind(`SELECT id, calendar_id, uid, href_name, etag, data, size, component, dtstart, dtend, updated_at
		FROM calendar_objects WHERE calendar_id = ?`)
	rows, err := s.db.QueryContext(ctx, q, calendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CalendarObject
	for rows.Next() {
		o, err := s.scanCalObject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) GetCalendarObject(ctx context.Context, calendarID, hrefName string) (*CalendarObject, error) {
	q := s.rebind(`SELECT id, calendar_id, uid, href_name, etag, data, size, component, dtstart, dtend, updated_at
		FROM calendar_objects WHERE calendar_id = ? AND href_name = ?`)
	return s.scanCalObject(s.db.QueryRowContext(ctx, q, calendarID, hrefName))
}

func (s *Store) CalendarObjectExistsByUID(ctx context.Context, calendarID, uid string) (bool, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return false, nil
	}
	q := s.rebind(`SELECT 1 FROM calendar_objects WHERE calendar_id = ? AND uid = ? LIMIT 1`)
	var one int
	err := s.db.QueryRowContext(ctx, q, calendarID, uid).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) AddressObjectExistsByUID(ctx context.Context, bookID, uid string) (bool, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return false, nil
	}
	q := s.rebind(`SELECT 1 FROM address_objects WHERE addressbook_id = ? AND uid = ? LIMIT 1`)
	var one int
	err := s.db.QueryRowContext(ctx, q, bookID, uid).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) GetCalendarObjectByID(ctx context.Context, id string) (*CalendarObject, error) {
	q := s.rebind(`SELECT id, calendar_id, uid, href_name, etag, data, size, component, dtstart, dtend, updated_at
		FROM calendar_objects WHERE id = ?`)
	return s.scanCalObject(s.db.QueryRowContext(ctx, q, id))
}

func (s *Store) UpsertCalendarObject(ctx context.Context, o *CalendarObject) (*CalendarObject, error) {
	now := time.Now().UTC()
	o.UpdatedAt = now
	o.ETag = ContentETag(o.Data)
	o.Size = int64(len(o.Data))
	existing, err := s.GetCalendarObject(ctx, o.CalendarID, o.HrefName)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	if err == ErrNotFound {
		if o.ID == "" {
			o.ID = NewID()
		}
		q := s.rebind(`INSERT INTO calendar_objects (id, calendar_id, uid, href_name, etag, data, size, component, dtstart, dtend, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
		_, err = s.db.ExecContext(ctx, q, o.ID, o.CalendarID, o.UID, o.HrefName, o.ETag, o.Data, o.Size, o.Component, o.DTStart, o.DTEnd, now)
	} else {
		o.ID = existing.ID
		q := s.rebind(`UPDATE calendar_objects SET uid=?, etag=?, data=?, size=?, component=?, dtstart=?, dtend=?, updated_at=? WHERE id=?`)
		_, err = s.db.ExecContext(ctx, q, o.UID, o.ETag, o.Data, o.Size, o.Component, o.DTStart, o.DTEnd, now, o.ID)
	}
	if err != nil {
		return nil, err
	}
	if err := s.bumpCalendarCTag(ctx, o.CalendarID); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *Store) DeleteCalendarObject(ctx context.Context, calendarID, hrefName string) error {
	q := s.rebind(`DELETE FROM calendar_objects WHERE calendar_id = ? AND href_name = ?`)
	res, err := s.db.ExecContext(ctx, q, calendarID, hrefName)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return s.bumpCalendarCTag(ctx, calendarID)
}

func (s *Store) scanCalObject(row interface{ Scan(dest ...any) error }) (*CalendarObject, error) {
	o := &CalendarObject{}
	var dtStart, dtEnd sql.NullTime
	err := row.Scan(&o.ID, &o.CalendarID, &o.UID, &o.HrefName, &o.ETag, &o.Data, &o.Size, &o.Component, &dtStart, &dtEnd, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if dtStart.Valid {
		t := dtStart.Time.UTC()
		o.DTStart = &t
	}
	if dtEnd.Valid {
		t := dtEnd.Time.UTC()
		o.DTEnd = &t
	}
	return o, nil
}

func (s *Store) EnsureAddressBook(ctx context.Context, userID, name, displayName string) (*AddressBook, error) {
	ab, err := s.GetAddressBookByName(ctx, userID, name)
	if err == nil {
		return ab, nil
	}
	if err != ErrNotFound {
		return nil, err
	}
	if displayName == "" {
		displayName = name
	}
	return s.CreateAddressBook(ctx, &AddressBook{
		UserID:      userID,
		Name:        name,
		DisplayName: displayName,
		CTag:        newCTag(),
	})
}

func (s *Store) CreateAddressBook(ctx context.Context, ab *AddressBook) (*AddressBook, error) {
	now := time.Now().UTC()
	ab.CreatedAt = now
	if ab.ID == "" {
		ab.ID = NewID()
	}
	if ab.CTag == "" {
		ab.CTag = newCTag()
	}
	q := s.rebind(`INSERT INTO addressbooks (id, user_id, name, display_name, description, ctag, created_at)
		VALUES (?,?,?,?,?,?,?)`)
	_, err := s.db.ExecContext(ctx, q, ab.ID, ab.UserID, ab.Name, ab.DisplayName, ab.Description, ab.CTag, now)
	return ab, err
}

func (s *Store) ListAddressBooks(ctx context.Context, userID string) ([]*AddressBook, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, description, ctag, created_at FROM addressbooks WHERE user_id = ? ORDER BY name`)
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AddressBook
	for rows.Next() {
		ab := &AddressBook{}
		if err := rows.Scan(&ab.ID, &ab.UserID, &ab.Name, &ab.DisplayName, &ab.Description, &ab.CTag, &ab.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ab)
	}
	return out, rows.Err()
}

func (s *Store) GetAddressBookByName(ctx context.Context, userID, name string) (*AddressBook, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, description, ctag, created_at FROM addressbooks WHERE user_id = ? AND name = ?`)
	ab := &AddressBook{}
	err := s.db.QueryRowContext(ctx, q, userID, name).Scan(&ab.ID, &ab.UserID, &ab.Name, &ab.DisplayName, &ab.Description, &ab.CTag, &ab.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return ab, nil
}

func (s *Store) GetAddressBookByID(ctx context.Context, userID, id string) (*AddressBook, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, description, ctag, created_at FROM addressbooks WHERE user_id = ? AND id = ?`)
	ab := &AddressBook{}
	err := s.db.QueryRowContext(ctx, q, userID, id).Scan(&ab.ID, &ab.UserID, &ab.Name, &ab.DisplayName, &ab.Description, &ab.CTag, &ab.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return ab, nil
}

func (s *Store) DeleteAddressBook(ctx context.Context, userID, name string) error {
	q := s.rebind(`DELETE FROM addressbooks WHERE user_id = ? AND name = ?`)
	res, err := s.db.ExecContext(ctx, q, userID, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) bumpAddressBookCTag(ctx context.Context, abID string) error {
	q := s.rebind(`UPDATE addressbooks SET ctag = ? WHERE id = ?`)
	_, err := s.db.ExecContext(ctx, q, newCTag(), abID)
	return err
}

func (s *Store) ListAddressObjects(ctx context.Context, addressBookID string) ([]*AddressObject, error) {
	q := s.rebind(`SELECT id, addressbook_id, uid, href_name, etag, data, size, updated_at FROM address_objects WHERE addressbook_id = ?`)
	rows, err := s.db.QueryContext(ctx, q, addressBookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AddressObject
	for rows.Next() {
		o, err := s.scanAddrObject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) GetAddressObject(ctx context.Context, addressBookID, hrefName string) (*AddressObject, error) {
	q := s.rebind(`SELECT id, addressbook_id, uid, href_name, etag, data, size, updated_at FROM address_objects WHERE addressbook_id = ? AND href_name = ?`)
	return s.scanAddrObject(s.db.QueryRowContext(ctx, q, addressBookID, hrefName))
}

func (s *Store) GetAddressObjectByID(ctx context.Context, id string) (*AddressObject, error) {
	q := s.rebind(`SELECT id, addressbook_id, uid, href_name, etag, data, size, updated_at FROM address_objects WHERE id = ?`)
	return s.scanAddrObject(s.db.QueryRowContext(ctx, q, id))
}

func (s *Store) UpsertAddressObject(ctx context.Context, o *AddressObject) (*AddressObject, error) {
	now := time.Now().UTC()
	o.UpdatedAt = now
	o.ETag = ContentETag(o.Data)
	o.Size = int64(len(o.Data))
	existing, err := s.GetAddressObject(ctx, o.AddressBookID, o.HrefName)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	if err == ErrNotFound {
		if o.ID == "" {
			o.ID = NewID()
		}
		q := s.rebind(`INSERT INTO address_objects (id, addressbook_id, uid, href_name, etag, data, size, updated_at)
			VALUES (?,?,?,?,?,?,?,?)`)
		_, err = s.db.ExecContext(ctx, q, o.ID, o.AddressBookID, o.UID, o.HrefName, o.ETag, o.Data, o.Size, now)
	} else {
		o.ID = existing.ID
		q := s.rebind(`UPDATE address_objects SET uid=?, etag=?, data=?, size=?, updated_at=? WHERE id=?`)
		_, err = s.db.ExecContext(ctx, q, o.UID, o.ETag, o.Data, o.Size, now, o.ID)
	}
	if err != nil {
		return nil, err
	}
	if err := s.bumpAddressBookCTag(ctx, o.AddressBookID); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *Store) DeleteAddressObject(ctx context.Context, addressBookID, hrefName string) error {
	q := s.rebind(`DELETE FROM address_objects WHERE addressbook_id = ? AND href_name = ?`)
	res, err := s.db.ExecContext(ctx, q, addressBookID, hrefName)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return s.bumpAddressBookCTag(ctx, addressBookID)
}

func (s *Store) scanAddrObject(row interface{ Scan(dest ...any) error }) (*AddressObject, error) {
	o := &AddressObject{}
	err := row.Scan(&o.ID, &o.AddressBookID, &o.UID, &o.HrefName, &o.ETag, &o.Data, &o.Size, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// EnsureDAVDefaults creates default calendar and address book for a user.
// The default calendar is named "default" (href) with display name "Календарь".
func (s *Store) EnsureDAVDefaults(ctx context.Context, userID string) error {
	if _, err := s.EnsureCalendar(ctx, userID, "default", "Календарь"); err != nil {
		return fmt.Errorf("ensure calendar: %w", err)
	}
	if _, err := s.EnsureAddressBook(ctx, userID, "default", "Contacts"); err != nil {
		return fmt.Errorf("ensure addressbook: %w", err)
	}
	return nil
}
