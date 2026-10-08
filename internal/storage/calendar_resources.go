package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Resource kinds for domain calendar resources.
const (
	ResourceKindRoom      = "room"
	ResourceKindEquipment = "equipment"
	ResourceKindOther     = "other"
)

// CalendarResource is a bookable domain asset (room, projector, mic, …).
type CalendarResource struct {
	ID          string
	TenantID    string
	DomainID    string
	UserID      string // backing user (auth_source=resource)
	Email       string
	LocalPart   string
	DisplayName string
	Kind        string
	Capacity    int
	Description string
	AutoAccept  bool
	Enabled     bool
	CreatedAt   time.Time
}

func normalizeResourceKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case ResourceKindEquipment, "equip", "device":
		return ResourceKindEquipment
	case ResourceKindOther, "":
		if kind == "" {
			return ResourceKindRoom
		}
		return ResourceKindOther
	case ResourceKindRoom, "meetingroom", "meeting-room":
		return ResourceKindRoom
	default:
		return ResourceKindOther
	}
}

func sanitizeResourceLocal(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), ".-_")
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

// CreateCalendarResource creates a backing user + default calendar + resource row.
func (s *Store) CreateCalendarResource(ctx context.Context, res *CalendarResource) (*CalendarResource, error) {
	if res == nil {
		return nil, fmt.Errorf("resource required")
	}
	dom, err := s.GetDomainByID(ctx, res.DomainID)
	if err != nil {
		return nil, err
	}
	res.TenantID = dom.TenantID
	res.Kind = normalizeResourceKind(res.Kind)
	local := sanitizeResourceLocal(res.LocalPart)
	if local == "" {
		local = sanitizeResourceLocal(res.DisplayName)
	}
	if local == "" {
		return nil, fmt.Errorf("local_part required")
	}
	if res.DisplayName == "" {
		res.DisplayName = local
	}
	email := strings.ToLower(local + "@" + dom.Name)
	if _, err := s.GetUserByEmail(ctx, email); err == nil {
		return nil, fmt.Errorf("email already exists")
	} else if err != ErrNotFound {
		return nil, err
	}
	if existing, err := s.GetCalendarResourceByEmail(ctx, email); err == nil && existing != nil {
		return nil, fmt.Errorf("resource already exists")
	} else if err != nil && err != ErrNotFound {
		return nil, err
	}

	u, err := s.CreateUser(ctx, &User{
		TenantID: dom.TenantID, DomainID: dom.ID,
		Email: email, LocalPart: local, DisplayName: res.DisplayName,
		AuthSource: "resource", Enabled: true, Roles: "",
		PasswordHash: "", // login blocked by auth_source
	})
	if err != nil {
		return nil, err
	}
	if _, err := s.EnsureCalendar(ctx, u.ID, "default", res.DisplayName); err != nil {
		_ = s.DeleteUser(ctx, u.ID)
		return nil, err
	}

	now := time.Now().UTC()
	res.ID = NewID()
	res.UserID = u.ID
	res.Email = email
	res.LocalPart = local
	res.CreatedAt = now
	res.Enabled = true // new resources start enabled; disable via Update
	auto := 1
	if !res.AutoAccept {
		auto = 0
	}
	en := 1
	q := s.rebind(`INSERT INTO calendar_resources
		(id, tenant_id, domain_id, user_id, email, local_part, display_name, kind, capacity, description, auto_accept, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if _, err := s.db.ExecContext(ctx, q,
		res.ID, res.TenantID, res.DomainID, res.UserID, res.Email, res.LocalPart, res.DisplayName,
		res.Kind, res.Capacity, res.Description, auto, en, now,
	); err != nil {
		_ = s.DeleteUser(ctx, u.ID)
		return nil, mapErr(err)
	}
	return res, nil
}

func (s *Store) UpdateCalendarResource(ctx context.Context, res *CalendarResource) error {
	res.Kind = normalizeResourceKind(res.Kind)
	auto := 1
	if !res.AutoAccept {
		auto = 0
	}
	en := 1
	if !res.Enabled {
		en = 0
	}
	q := s.rebind(`UPDATE calendar_resources SET display_name=?, kind=?, capacity=?, description=?, auto_accept=?, enabled=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx, q, res.DisplayName, res.Kind, res.Capacity, res.Description, auto, en, res.ID)
	if err != nil {
		return err
	}
	if res.UserID != "" && res.DisplayName != "" {
		_ = s.UpdateUserProfile(ctx, res.UserID, res.DisplayName)
	}
	if res.UserID != "" {
		_ = s.UpdateUserEnabled(ctx, res.UserID, res.Enabled)
	}
	return nil
}

func (s *Store) DeleteCalendarResource(ctx context.Context, id string) error {
	res, err := s.GetCalendarResource(ctx, id)
	if err != nil {
		return err
	}
	q := s.rebind(`DELETE FROM calendar_resources WHERE id = ?`)
	if _, err := s.db.ExecContext(ctx, q, id); err != nil {
		return err
	}
	if res.UserID != "" {
		_ = s.DeleteUser(ctx, res.UserID)
	}
	return nil
}

func (s *Store) GetCalendarResource(ctx context.Context, id string) (*CalendarResource, error) {
	q := s.rebind(`SELECT id, tenant_id, domain_id, user_id, email, local_part, display_name, kind, capacity, description, auto_accept, enabled, created_at
		FROM calendar_resources WHERE id = ?`)
	return s.scanCalendarResource(s.db.QueryRowContext(ctx, q, id))
}

func (s *Store) GetCalendarResourceByEmail(ctx context.Context, email string) (*CalendarResource, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	q := s.rebind(`SELECT id, tenant_id, domain_id, user_id, email, local_part, display_name, kind, capacity, description, auto_accept, enabled, created_at
		FROM calendar_resources WHERE email = ?`)
	return s.scanCalendarResource(s.db.QueryRowContext(ctx, q, email))
}

func (s *Store) GetCalendarResourceByUserID(ctx context.Context, userID string) (*CalendarResource, error) {
	q := s.rebind(`SELECT id, tenant_id, domain_id, user_id, email, local_part, display_name, kind, capacity, description, auto_accept, enabled, created_at
		FROM calendar_resources WHERE user_id = ?`)
	return s.scanCalendarResource(s.db.QueryRowContext(ctx, q, userID))
}

func (s *Store) ListCalendarResourcesByDomain(ctx context.Context, domainID string, enabledOnly bool) ([]*CalendarResource, error) {
	q := `SELECT id, tenant_id, domain_id, user_id, email, local_part, display_name, kind, capacity, description, auto_accept, enabled, created_at
		FROM calendar_resources WHERE domain_id = ?`
	if enabledOnly {
		q += ` AND enabled = 1`
	}
	q += ` ORDER BY kind, display_name`
	rows, err := s.db.QueryContext(ctx, s.rebind(q), domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanCalendarResourceRows(rows)
}

func (s *Store) ListCalendarResourcesByTenant(ctx context.Context, tenantID string, enabledOnly bool) ([]*CalendarResource, error) {
	q := `SELECT id, tenant_id, domain_id, user_id, email, local_part, display_name, kind, capacity, description, auto_accept, enabled, created_at
		FROM calendar_resources WHERE tenant_id = ?`
	if enabledOnly {
		q += ` AND enabled = 1`
	}
	q += ` ORDER BY kind, display_name`
	rows, err := s.db.QueryContext(ctx, s.rebind(q), tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanCalendarResourceRows(rows)
}

func (s *Store) scanCalendarResourceRows(rows *sql.Rows) ([]*CalendarResource, error) {
	var out []*CalendarResource
	for rows.Next() {
		res, err := s.scanCalendarResource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

func (s *Store) scanCalendarResource(row interface{ Scan(dest ...any) error }) (*CalendarResource, error) {
	res := &CalendarResource{}
	var auto, en any
	err := row.Scan(
		&res.ID, &res.TenantID, &res.DomainID, &res.UserID, &res.Email, &res.LocalPart,
		&res.DisplayName, &res.Kind, &res.Capacity, &res.Description, &auto, &en, &res.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	res.AutoAccept = asBool(auto)
	res.Enabled = asBool(en)
	return res, nil
}
