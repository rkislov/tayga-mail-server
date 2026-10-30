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

	"github.com/tayga/tms/internal/calutil"
)

// Calendar is a CalDAV collection owned by a user.
type Calendar struct {
	ID          string
	UserID      string
	Name        string
	DisplayName string
	Description string
	Color       string
	PublicToken string
	CTag        string
	CreatedAt   time.Time
}

// CalendarObject is an iCalendar resource inside a calendar.
type CalendarObject struct {
	// Optional write preconditions; not persisted or returned to clients.
	ExpectedETag string `json:"-"`
	CreateOnly   bool   `json:"-"`
	ID           string
	CalendarID   string
	UID          string
	HrefName     string
	ETag         string
	Data         string
	Size         int64
	Component    string
	DTStart      *time.Time
	DTEnd        *time.Time
	UpdatedAt    time.Time
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
	if c.Color == "" {
		c.Color = "#c77c35"
	}
	c.CreatedAt = now
	if c.ID == "" {
		c.ID = NewID()
	}
	if c.CTag == "" {
		c.CTag = newCTag()
	}
	q := s.rebind(`INSERT INTO calendars (id, user_id, name, display_name, description, color, public_token, ctag, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, q, c.ID, c.UserID, c.Name, c.DisplayName, c.Description, c.Color, c.PublicToken, c.CTag, now)
	return c, err
}

func (s *Store) ListCalendars(ctx context.Context, userID string) ([]*Calendar, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, description, color, public_token, ctag, created_at FROM calendars WHERE user_id = ? ORDER BY name`)
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Calendar
	for rows.Next() {
		c := &Calendar{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.DisplayName, &c.Description, &c.Color, &c.PublicToken, &c.CTag, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCalendarByName(ctx context.Context, userID, name string) (*Calendar, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, description, color, public_token, ctag, created_at FROM calendars WHERE user_id = ? AND name = ?`)
	c := &Calendar{}
	err := s.db.QueryRowContext(ctx, q, userID, name).Scan(&c.ID, &c.UserID, &c.Name, &c.DisplayName, &c.Description, &c.Color, &c.PublicToken, &c.CTag, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) GetCalendarByID(ctx context.Context, userID, id string) (*Calendar, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, description, color, public_token, ctag, created_at FROM calendars WHERE user_id = ? AND id = ?`)
	c := &Calendar{}
	err := s.db.QueryRowContext(ctx, q, userID, id).Scan(&c.ID, &c.UserID, &c.Name, &c.DisplayName, &c.Description, &c.Color, &c.PublicToken, &c.CTag, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) DeleteCalendar(ctx context.Context, userID, name string) error {
	tx, err := s.beginCalendarWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id string
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT id FROM calendars WHERE user_id=? AND name=?`), userID, name).Scan(&id); err != nil {
		return mapErr(err)
	}
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT id,calendar_id,uid,href_name,etag,data,size,component,dtstart,dtend,updated_at FROM calendar_objects WHERE calendar_id=?`), id)
	if err != nil {
		return err
	}
	var objects []*CalendarObject
	for rows.Next() {
		o, e := s.scanCalObject(rows)
		if e != nil {
			rows.Close()
			return e
		}
		objects = append(objects, o)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, o := range objects {
		p, e := calutil.ParseICS(o.Data)
		if e != nil {
			continue
		}
		previous := o.Data
		o.Data, e = calutil.BuildCancelICS(p.Event)
		if e != nil {
			return e
		}
		if e = s.bookCalendarResourcesTx(ctx, tx, o, previous); e != nil {
			return e
		}
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM calendars WHERE id=?`), id); err != nil {
		return err
	}
	return tx.Commit()
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

func (s *Store) GetCalendarObjectByUID(ctx context.Context, calendarID, uid string) (*CalendarObject, error) {
	q := s.rebind(`SELECT id,calendar_id,uid,href_name,etag,data,size,component,dtstart,dtend,updated_at FROM calendar_objects WHERE calendar_id=? AND uid=?`)
	return s.scanCalObject(s.db.QueryRowContext(ctx, q, calendarID, uid))
}

func (s *Store) UpsertCalendarObject(ctx context.Context, o *CalendarObject) (*CalendarObject, error) {
	return s.SaveCalendarObjectWithMail(ctx, o, nil)
}

func (s *Store) SaveCalendarObjectWithMail(ctx context.Context, o *CalendarObject, mails []CalendarMail) (*CalendarObject, error) {
	return s.saveCalendarObject(ctx, o, mails, "", false)
}

func (s *Store) saveCalendarObject(ctx context.Context, o *CalendarObject, mails []CalendarMail, actorID string, clientWrite bool) (*CalendarObject, error) {
	if actorID != "" {
		copy := *o
		o = &copy
	}
	tx, err := s.beginCalendarWrite(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var old string
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT data FROM calendar_objects WHERE calendar_id=? AND href_name=?`), o.CalendarID, o.HrefName).Scan(&old)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	bookResources := true
	if clientWrite {
		email, _, e := s.calendarWriterTx(ctx, tx, actorID, o.CalendarID)
		if e != nil {
			return nil, e
		}
		if old != "" {
			var component, uid string
			if e = tx.QueryRowContext(ctx, s.rebind(`SELECT component,uid FROM calendar_objects WHERE calendar_id=? AND href_name=?`), o.CalendarID, o.HrefName).Scan(&component, &uid); e != nil {
				return nil, e
			}
			if component != o.Component || uid != o.UID {
				return nil, ErrCalendarConflict
			}
		}
		if o.Component == "VEVENT" {
			var schedule bool
			o.Data, schedule, e = calutil.PrepareOrganizerWrite(o.Data, old, email)
			if e != nil {
				return nil, fmt.Errorf("%w: %v", ErrCalendarConflict, e)
			}
			if schedule && old == "" {
				rows, e := tx.QueryContext(ctx, s.rebind(`SELECT o.data FROM calendar_objects o JOIN calendars c ON c.id=o.calendar_id WHERE c.user_id=? AND o.uid=?`), actorID, o.UID)
				if e != nil {
					return nil, e
				}
				duplicate := false
				for rows.Next() {
					var data string
					if e = rows.Scan(&data); e != nil {
						break
					}
					p, parseErr := calutil.ParseICS(data)
					if parseErr == nil && strings.EqualFold(p.Organizer.Email, email) {
						duplicate = true
					}
				}
				rowErr := rows.Err()
				rows.Close()
				if e != nil {
					return nil, e
				}
				if rowErr != nil {
					return nil, rowErr
				}
				if duplicate {
					return nil, ErrCalendarConflict
				}
			}
			if !schedule {
				actorID = ""
				// Personal/recipient changes never schedule on behalf of another
				// organizer, even when that organizer has an ACL on this calendar.
				// Direct edits of a resource calendar still validate availability.
				var direct int
				if e = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM calendar_resources r JOIN calendars c ON c.user_id=r.user_id WHERE c.id=?`), o.CalendarID).Scan(&direct); e != nil {
					return nil, e
				}
				bookResources = direct > 0
			}
		} else {
			actorID = ""
		}
	}
	if err = s.saveCalendarObjectTx(ctx, tx, o); err != nil {
		return nil, err
	}
	if bookResources {
		if err = s.bookCalendarResourcesTx(ctx, tx, o, old); err != nil {
			return nil, err
		}
	}
	if actorID != "" {
		mails, err = s.syncCalendarMeetingTx(ctx, tx, actorID, o, old)
		if err != nil {
			return nil, err
		}
	}
	// The queue must contain the same accepted resource statuses as the event.
	for i := range mails {
		if mails[i].Method == "REQUEST" {
			p, e := calutil.ParseICS(mails[i].ICS)
			if e == nil && p.UID == o.UID {
				updated, e := calutil.ParseICS(o.Data)
				if e != nil {
					return nil, e
				}
				mails[i].ICS, e = calutil.BuildRequestICS(updated.Event)
				if e != nil {
					return nil, e
				}
			}
		}
	}
	if err = s.queueCalendarMail(ctx, tx, mails); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	o.ExpectedETag, o.CreateOnly = "", false
	return o, nil
}

// Internal writes share the caller's scheduling transaction.
func (s *Store) saveCalendarObjectTx(ctx context.Context, tx *sql.Tx, o *CalendarObject) error {
	now := time.Now().UTC()
	o.UpdatedAt = now
	o.ETag = ContentETag(o.Data)
	o.Size = int64(len(o.Data))
	existing, err := s.scanCalObject(tx.QueryRowContext(ctx, s.rebind(`SELECT id,calendar_id,uid,href_name,etag,data,size,component,dtstart,dtend,updated_at FROM calendar_objects WHERE calendar_id=? AND href_name=?`), o.CalendarID, o.HrefName))
	if err != nil && err != ErrNotFound {
		return err
	}
	if err == ErrNotFound {
		if o.ExpectedETag != "" {
			return ErrCalendarConflict
		}
		if o.ID == "" {
			o.ID = NewID()
		}
		q := s.rebind(`INSERT INTO calendar_objects (id, calendar_id, uid, href_name, etag, data, size, component, dtstart, dtend, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
		_, err = tx.ExecContext(ctx, q, o.ID, o.CalendarID, o.UID, o.HrefName, o.ETag, o.Data, o.Size, o.Component, o.DTStart, o.DTEnd, now)
	} else {
		if o.CreateOnly {
			return ErrCalendarConflict
		}
		if o.ExpectedETag != "" && o.ExpectedETag != existing.ETag {
			return ErrCalendarConflict
		}
		if o.Data == existing.Data && o.UID == existing.UID && o.Component == existing.Component {
			*o = *existing
			return nil
		}
		o.ID = existing.ID
		q := `UPDATE calendar_objects SET uid=?, etag=?, data=?, size=?, component=?, dtstart=?, dtend=?, updated_at=? WHERE id=?`
		args := []any{o.UID, o.ETag, o.Data, o.Size, o.Component, o.DTStart, o.DTEnd, now, o.ID}
		if o.ExpectedETag != "" {
			q += ` AND etag=?`
			args = append(args, o.ExpectedETag)
		}
		var result sql.Result
		result, err = tx.ExecContext(ctx, s.rebind(q), args...)
		if err == nil {
			n, e := result.RowsAffected()
			if e != nil {
				return e
			}
			if n != 1 {
				return ErrCalendarConflict
			}
		}
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, s.rebind(`UPDATE calendars SET ctag=? WHERE id=?`), newCTag(), o.CalendarID); err != nil {
		return err
	}
	return nil
}

func (s *Store) DeleteCalendarObject(ctx context.Context, calendarID, hrefName string) error {
	o, err := s.GetCalendarObject(ctx, calendarID, hrefName)
	if err != nil {
		return err
	}
	return s.DeleteCalendarObjectWithMail(ctx, o, nil)
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
	for _, entry := range []struct{ name, display string }{{"personal", "Personal calendar"}, {"work", "Work calendar"}} {
		if _, err := s.EnsureCalendar(ctx, userID, entry.name, entry.display); err != nil {
			return fmt.Errorf("ensure calendar: %w", err)
		}
	}
	if _, err := s.EnsureAddressBook(ctx, userID, "default", "Contacts"); err != nil {
		return fmt.Errorf("ensure addressbook: %w", err)
	}
	return nil
}

func (s *Store) UpdateCalendar(ctx context.Context, c *Calendar) error {
	res, err := s.db.ExecContext(ctx, s.rebind(`UPDATE calendars SET display_name=?, description=?, color=?, public_token=?, ctag=? WHERE id=? AND user_id=?`), c.DisplayName, c.Description, c.Color, c.PublicToken, newCTag(), c.ID, c.UserID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) GetPublicCalendar(ctx context.Context, token string) (*Calendar, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	c := &Calendar{}
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT id,user_id,name,display_name,description,color,public_token,ctag,created_at FROM calendars WHERE public_token=?`), token).Scan(&c.ID, &c.UserID, &c.Name, &c.DisplayName, &c.Description, &c.Color, &c.PublicToken, &c.CTag, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return c, err
}
