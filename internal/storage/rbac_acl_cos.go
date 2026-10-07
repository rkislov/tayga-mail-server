package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"strings"
	"time"
)

func (s *Store) UpdateUserServiceClass(ctx context.Context, userID, serviceClassID string) error {
	q := s.rebind(`UPDATE users SET service_class_id = ? WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, serviceClassID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateUserMigration(ctx context.Context, userID, migrationEnabled string) error {
	migrationEnabled = normalizeMigrationPolicy(migrationEnabled)
	q := s.rebind(`UPDATE users SET migration_enabled = ? WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, migrationEnabled, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListUsersByDomain(ctx context.Context, domainID string) ([]*User, error) {
	q := s.rebind(`SELECT ` + userCols + ` FROM users WHERE domain_id = ? ORDER BY email`)
	rows, err := s.db.QueryContext(ctx, q, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := s.scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) SetDomainAdminDomains(ctx context.Context, userID string, domainIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, s.rebind(`DELETE FROM domain_admin_domains WHERE user_id = ?`), userID); err != nil {
		return err
	}
	for _, id := range domainIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO domain_admin_domains(user_id, domain_id) VALUES (?, ?)`), userID, id); err != nil {
			return mapErr(err)
		}
	}
	return tx.Commit()
}

func (s *Store) ListDomainAdminDomains(ctx context.Context, userID string) ([]string, error) {
	q := s.rebind(`SELECT domain_id FROM domain_admin_domains WHERE user_id = ?`)
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) UserAdministersDomain(ctx context.Context, userID, domainID string) (bool, error) {
	u, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}
	if IsGlobalAdmin(u.Roles) {
		return true, nil
	}
	if !HasRole(u.Roles, RoleDomainAdmin) {
		return false, nil
	}
	var n int
	err = s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM domain_admin_domains WHERE user_id = ? AND domain_id = ?`), userID, domainID).Scan(&n)
	return n > 0, err
}

func randomShareToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Store) CreateFileShare(ctx context.Context, sh *FileShare) (*FileShare, error) {
	if sh.ID == "" {
		sh.ID = NewID()
	}
	if sh.Token == "" {
		tok, err := randomShareToken()
		if err != nil {
			return nil, err
		}
		sh.Token = tok
	}
	sh.CreatedAt = time.Now().UTC()
	q := s.rebind(`INSERT INTO file_shares(id, user_id, path, token, expires_at, max_downloads, download_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?)`)
	if _, err := s.db.ExecContext(ctx, q, sh.ID, sh.UserID, sh.Path, sh.Token, sh.ExpiresAt, sh.MaxDownloads, sh.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return sh, nil
}

func (s *Store) scanFileShare(row interface{ Scan(dest ...any) error }) (*FileShare, error) {
	sh := &FileShare{}
	var exp sql.NullTime
	err := row.Scan(&sh.ID, &sh.UserID, &sh.Path, &sh.Token, &exp, &sh.MaxDownloads, &sh.DownloadCount, &sh.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	if exp.Valid {
		t := exp.Time.UTC()
		sh.ExpiresAt = &t
	}
	return sh, nil
}

func (s *Store) GetFileShareByToken(ctx context.Context, token string) (*FileShare, error) {
	q := s.rebind(`SELECT id, user_id, path, token, expires_at, max_downloads, download_count, created_at FROM file_shares WHERE token = ?`)
	return s.scanFileShare(s.db.QueryRowContext(ctx, q, token))
}

func (s *Store) GetFileShareByID(ctx context.Context, id string) (*FileShare, error) {
	q := s.rebind(`SELECT id, user_id, path, token, expires_at, max_downloads, download_count, created_at FROM file_shares WHERE id = ?`)
	return s.scanFileShare(s.db.QueryRowContext(ctx, q, id))
}

func (s *Store) ListFileShares(ctx context.Context, userID string) ([]*FileShare, error) {
	q := s.rebind(`SELECT id, user_id, path, token, expires_at, max_downloads, download_count, created_at FROM file_shares WHERE user_id = ? ORDER BY created_at DESC`)
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*FileShare
	for rows.Next() {
		sh, err := s.scanFileShare(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

func (s *Store) DeleteFileShare(ctx context.Context, userID, id string) error {
	res, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM file_shares WHERE id = ? AND user_id = ?`), id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) IncrementFileShareDownload(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE file_shares SET download_count = download_count + 1 WHERE id = ?`), id)
	return err
}

func (s *Store) UpsertMailboxDelegate(ctx context.Context, d *MailboxDelegate) error {
	q := s.rebind(`INSERT INTO mailbox_delegates(owner_id, delegate_id, can_read, can_send_as, can_send_on_behalf)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(owner_id, delegate_id) DO UPDATE SET
			can_read = excluded.can_read,
			can_send_as = excluded.can_send_as,
			can_send_on_behalf = excluded.can_send_on_behalf`)
	_, err := s.db.ExecContext(ctx, q, d.OwnerID, d.DelegateID, d.CanRead, d.CanSendAs, d.CanSendOnBehalf)
	return mapErr(err)
}

func (s *Store) scanDelegate(row interface{ Scan(dest ...any) error }) (*MailboxDelegate, error) {
	d := &MailboxDelegate{}
	var cr, csa, csob any
	if err := row.Scan(&d.OwnerID, &d.DelegateID, &cr, &csa, &csob); err != nil {
		return nil, mapErr(err)
	}
	d.CanRead, d.CanSendAs, d.CanSendOnBehalf = asBool(cr), asBool(csa), asBool(csob)
	return d, nil
}

func (s *Store) ListMailboxDelegates(ctx context.Context, ownerID string) ([]*MailboxDelegate, error) {
	q := s.rebind(`SELECT owner_id, delegate_id, can_read, can_send_as, can_send_on_behalf FROM mailbox_delegates WHERE owner_id = ?`)
	return s.queryDelegates(ctx, q, ownerID)
}

func (s *Store) ListDelegationsFor(ctx context.Context, delegateID string) ([]*MailboxDelegate, error) {
	q := s.rebind(`SELECT owner_id, delegate_id, can_read, can_send_as, can_send_on_behalf FROM mailbox_delegates WHERE delegate_id = ?`)
	return s.queryDelegates(ctx, q, delegateID)
}

func (s *Store) queryDelegates(ctx context.Context, q string, arg string) ([]*MailboxDelegate, error) {
	rows, err := s.db.QueryContext(ctx, q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*MailboxDelegate
	for rows.Next() {
		d, err := s.scanDelegate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetMailboxDelegate(ctx context.Context, ownerID, delegateID string) (*MailboxDelegate, error) {
	q := s.rebind(`SELECT owner_id, delegate_id, can_read, can_send_as, can_send_on_behalf FROM mailbox_delegates WHERE owner_id = ? AND delegate_id = ?`)
	return s.scanDelegate(s.db.QueryRowContext(ctx, q, ownerID, delegateID))
}

func (s *Store) DeleteMailboxDelegate(ctx context.Context, ownerID, delegateID string) error {
	res, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM mailbox_delegates WHERE owner_id = ? AND delegate_id = ?`), ownerID, delegateID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetMailboxACL(ctx context.Context, mailboxID, granteeUserID, rights string) error {
	rights = strings.TrimSpace(rights)
	if rights == "" {
		return s.DeleteMailboxACL(ctx, mailboxID, granteeUserID)
	}
	q := s.rebind(`INSERT INTO mailbox_acl(mailbox_id, grantee_user_id, rights) VALUES (?, ?, ?)
		ON CONFLICT(mailbox_id, grantee_user_id) DO UPDATE SET rights = excluded.rights`)
	_, err := s.db.ExecContext(ctx, q, mailboxID, granteeUserID, rights)
	return mapErr(err)
}

func (s *Store) DeleteMailboxACL(ctx context.Context, mailboxID, granteeUserID string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM mailbox_acl WHERE mailbox_id = ? AND grantee_user_id = ?`), mailboxID, granteeUserID)
	return err
}

func (s *Store) ListMailboxACL(ctx context.Context, mailboxID string) ([]*MailboxACLEntry, error) {
	q := s.rebind(`SELECT mailbox_id, grantee_user_id, rights FROM mailbox_acl WHERE mailbox_id = ?`)
	rows, err := s.db.QueryContext(ctx, q, mailboxID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*MailboxACLEntry
	for rows.Next() {
		e := &MailboxACLEntry{}
		if err := rows.Scan(&e.MailboxID, &e.GranteeUserID, &e.Rights); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) MailboxRightsForUser(ctx context.Context, mailboxID, userID string) (string, error) {
	mb, err := s.GetMailboxByID(ctx, mailboxID)
	if err != nil {
		return "", err
	}
	if mb.UserID == userID {
		return "lrswipcdkxta", nil
	}
	if d, err := s.GetMailboxDelegate(ctx, mb.UserID, userID); err == nil && d.CanRead {
		return "lrs", nil
	}
	var rights string
	err = s.db.QueryRowContext(ctx, s.rebind(`SELECT rights FROM mailbox_acl WHERE mailbox_id = ? AND grantee_user_id = ?`), mailboxID, userID).Scan(&rights)
	if err != nil {
		return "", mapErr(err)
	}
	return rights, nil
}

func (s *Store) ListSharedMailboxes(ctx context.Context, userID string) ([]*Mailbox, error) {
	q := s.rebind(`SELECT DISTINCT m.id, m.user_id, m.name, m.path, m.uidnext, m.uidvalidity, m.created_at
		FROM mailboxes m
		WHERE m.id IN (SELECT mailbox_id FROM mailbox_acl WHERE grantee_user_id = ?)
		   OR m.user_id IN (SELECT owner_id FROM mailbox_delegates WHERE delegate_id = ? AND can_read)`)
	rows, err := s.db.QueryContext(ctx, q, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Mailbox
	for rows.Next() {
		mb := &Mailbox{}
		if err := rows.Scan(&mb.ID, &mb.UserID, &mb.Name, &mb.Path, &mb.UIDNext, &mb.UIDValidity, &mb.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, mb)
	}
	return out, rows.Err()
}

func (s *Store) SetCalendarACL(ctx context.Context, calendarID, granteeUserID, rights string) error {
	rights = strings.TrimSpace(rights)
	if rights == "" {
		return s.DeleteCalendarACL(ctx, calendarID, granteeUserID)
	}
	q := s.rebind(`INSERT INTO calendar_acl(calendar_id, grantee_user_id, rights) VALUES (?, ?, ?)
		ON CONFLICT(calendar_id, grantee_user_id) DO UPDATE SET rights = excluded.rights`)
	_, err := s.db.ExecContext(ctx, q, calendarID, granteeUserID, rights)
	return mapErr(err)
}

func (s *Store) DeleteCalendarACL(ctx context.Context, calendarID, granteeUserID string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM calendar_acl WHERE calendar_id = ? AND grantee_user_id = ?`), calendarID, granteeUserID)
	return err
}

func (s *Store) ListCalendarACL(ctx context.Context, calendarID string) ([]*CalendarACLEntry, error) {
	q := s.rebind(`SELECT calendar_id, grantee_user_id, rights FROM calendar_acl WHERE calendar_id = ?`)
	rows, err := s.db.QueryContext(ctx, q, calendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CalendarACLEntry
	for rows.Next() {
		e := &CalendarACLEntry{}
		if err := rows.Scan(&e.CalendarID, &e.GranteeUserID, &e.Rights); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) CalendarRightsForUser(ctx context.Context, calendarID, userID string) (string, error) {
	var owner string
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT user_id FROM calendars WHERE id = ?`), calendarID).Scan(&owner)
	if err != nil {
		return "", mapErr(err)
	}
	if owner == userID {
		return "write", nil
	}
	var rights string
	err = s.db.QueryRowContext(ctx, s.rebind(`SELECT rights FROM calendar_acl WHERE calendar_id = ? AND grantee_user_id = ?`), calendarID, userID).Scan(&rights)
	if err != nil {
		return "", mapErr(err)
	}
	return rights, nil
}

func (s *Store) ListSharedCalendars(ctx context.Context, userID string) ([]*Calendar, error) {
	q := s.rebind(`SELECT c.id, c.user_id, c.name, c.display_name, c.description, c.ctag, c.created_at
		FROM calendars c
		INNER JOIN calendar_acl a ON a.calendar_id = c.id
		WHERE a.grantee_user_id = ?`)
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

func (s *Store) CreateServiceClass(ctx context.Context, sc *ServiceClass) (*ServiceClass, error) {
	if sc.ID == "" {
		sc.ID = NewID()
	}
	if sc.Config == "" {
		sc.Config = "{}"
	}
	sc.CreatedAt = time.Now().UTC()
	q := s.rebind(`INSERT INTO service_classes(id, tenant_id, name, config, created_at) VALUES (?, ?, ?, ?, ?)`)
	if _, err := s.db.ExecContext(ctx, q, sc.ID, sc.TenantID, sc.Name, sc.Config, sc.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return sc, nil
}

func (s *Store) UpdateServiceClass(ctx context.Context, sc *ServiceClass) error {
	q := s.rebind(`UPDATE service_classes SET name = ?, config = ? WHERE id = ? AND tenant_id = ?`)
	res, err := s.db.ExecContext(ctx, q, sc.Name, sc.Config, sc.ID, sc.TenantID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteServiceClass(ctx context.Context, tenantID, id string) error {
	res, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM service_classes WHERE id = ? AND tenant_id = ?`), id, tenantID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetServiceClass(ctx context.Context, id string) (*ServiceClass, error) {
	sc := &ServiceClass{}
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT id, tenant_id, name, config, created_at FROM service_classes WHERE id = ?`), id).
		Scan(&sc.ID, &sc.TenantID, &sc.Name, &sc.Config, &sc.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return sc, nil
}

func (s *Store) ListServiceClasses(ctx context.Context, tenantID string) ([]*ServiceClass, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id, tenant_id, name, config, created_at FROM service_classes WHERE tenant_id = ? ORDER BY name`), tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ServiceClass
	for rows.Next() {
		sc := &ServiceClass{}
		if err := rows.Scan(&sc.ID, &sc.TenantID, &sc.Name, &sc.Config, &sc.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}
