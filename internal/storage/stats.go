package storage

import (
	"context"
)

// ServerStats is aggregate counters for monitoring.
type ServerStats struct {
	Tenants      int   `json:"tenants"`
	Domains      int   `json:"domains"`
	Users        int   `json:"users"`
	UsersEnabled int   `json:"users_enabled"`
	Mailboxes    int   `json:"mailboxes"`
	Messages     int   `json:"messages"`
	BytesStored  int64 `json:"bytes_stored"`
	Calendars    int   `json:"calendars"`
	Contacts     int   `json:"contacts"` // address objects
	SieveScripts int   `json:"sieve_scripts"`
}

// TenantStats is per-tenant monitoring summary.
type TenantStats struct {
	TenantID     string `json:"tenant_id"`
	TenantName   string `json:"tenant_name"`
	Domains      int    `json:"domains"`
	Users        int    `json:"users"`
	UsersEnabled int    `json:"users_enabled"`
	Messages     int    `json:"messages"`
	BytesStored  int64  `json:"bytes_stored"`
}

func (s *Store) ServerStats(ctx context.Context) (*ServerStats, error) {
	st := &ServerStats{}
	queries := []struct {
		q string
		n *int
	}{
		{`SELECT COUNT(*) FROM tenants`, &st.Tenants},
		{`SELECT COUNT(*) FROM domains`, &st.Domains},
		{`SELECT COUNT(*) FROM users`, &st.Users},
		{`SELECT COUNT(*) FROM mailboxes`, &st.Mailboxes},
		{`SELECT COUNT(*) FROM messages`, &st.Messages},
		{`SELECT COUNT(*) FROM calendars`, &st.Calendars},
		{`SELECT COUNT(*) FROM address_objects`, &st.Contacts},
		{`SELECT COUNT(*) FROM sieve_scripts`, &st.SieveScripts},
	}
	for _, item := range queries {
		if err := s.db.QueryRowContext(ctx, s.rebind(item.q)).Scan(item.n); err != nil {
			// Some tables may be absent on very old DBs; ignore and leave 0.
			continue
		}
	}
	_ = s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM users WHERE enabled = ?`), 1).Scan(&st.UsersEnabled)
	_ = s.db.QueryRowContext(ctx, s.rebind(`SELECT COALESCE(SUM(size), 0) FROM messages`)).Scan(&st.BytesStored)
	return st, nil
}

func (s *Store) TenantStats(ctx context.Context, tenantID string) (*TenantStats, error) {
	t, err := s.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	st := &TenantStats{TenantID: t.ID, TenantName: t.Name}
	_ = s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM domains WHERE tenant_id = ?`), tenantID).Scan(&st.Domains)
	_ = s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM users WHERE tenant_id = ?`), tenantID).Scan(&st.Users)
	_ = s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM users WHERE tenant_id = ? AND enabled = ?`), tenantID, 1).Scan(&st.UsersEnabled)
	q := s.rebind(`
		SELECT COUNT(m.id), COALESCE(SUM(m.size), 0)
		FROM messages m
		JOIN mailboxes mb ON mb.id = m.mailbox_id
		JOIN users u ON u.id = mb.user_id
		WHERE u.tenant_id = ?`)
	_ = s.db.QueryRowContext(ctx, q, tenantID).Scan(&st.Messages, &st.BytesStored)
	return st, nil
}
