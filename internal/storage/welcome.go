package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrWelcomeConflict = errors.New("welcome template changed; reload before saving")
var ErrWelcomeInvalid = errors.New("invalid welcome template action")

// WelcomeRevision is immutable. Restoring a revision appends a copy.
type WelcomeRevision struct {
	Number       int64     `json:"number"`
	Subject      string    `json:"subject"`
	HTML         string    `json:"html"`
	Text         string    `json:"text"`
	Comment      string    `json:"comment"`
	Author       string    `json:"author"`
	SavedAt      time.Time `json:"saved_at"`
	RestoredFrom int64     `json:"restored_from,omitempty"`
}

type WelcomeEvent struct {
	Action   string    `json:"action"`
	Revision int64     `json:"revision"`
	Mode     string    `json:"mode"`
	Actor    string    `json:"actor"`
	At       time.Time `json:"at"`
}

type WelcomeTemplate struct {
	Kind       string            `json:"kind"`
	ScopeID    string            `json:"scope_id"`
	Generation int64             `json:"generation"`
	Mode       string            `json:"mode"`
	Published  int64             `json:"published"`
	Revisions  []WelcomeRevision `json:"revisions"`
	Events     []WelcomeEvent    `json:"events"`
}

type WelcomeChange struct {
	Expected int64           `json:"expected"`
	Action   string          `json:"action"`
	Revision int64           `json:"revision"`
	Mode     string          `json:"mode"`
	Content  WelcomeRevision `json:"content"`
}

func emptyWelcome(kind, id string) *WelcomeTemplate {
	mode := "off"
	if kind == "domains" {
		mode = "inherit"
	}
	return &WelcomeTemplate{Kind: kind, ScopeID: id, Mode: mode, Revisions: []WelcomeRevision{}, Events: []WelcomeEvent{}}
}

func (s *Store) GetWelcomeTemplate(ctx context.Context, kind, id string) (*WelcomeTemplate, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT snapshot FROM welcome_templates WHERE kind = ? AND scope_id = ?`), kind, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return emptyWelcome(kind, id), nil
	}
	if err != nil {
		return nil, err
	}
	var v WelcomeTemplate
	if err = json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// ChangeWelcomeTemplate uses a database compare-and-swap, including first creation.
// All metadata, immutable revisions and publication change in one statement.
func (s *Store) ChangeWelcomeTemplate(ctx context.Context, kind, id, actor string, c WelcomeChange) (*WelcomeTemplate, error) {
	v, err := s.GetWelcomeTemplate(ctx, kind, id)
	if err != nil {
		return nil, err
	}
	if v.Generation != c.Expected {
		return nil, ErrWelcomeConflict
	}
	var selected *WelcomeRevision
	for i := range v.Revisions {
		if v.Revisions[i].Number == c.Revision {
			selected = &v.Revisions[i]
			break
		}
	}
	now := time.Now().UTC()
	switch c.Action {
	case "save", "restore":
		content := c.Content
		if c.Action == "restore" {
			if selected == nil {
				return nil, ErrNotFound
			}
			content = *selected
			content.RestoredFrom = selected.Number
			content.Comment = c.Content.Comment
		} else {
			content.RestoredFrom = 0
		}
		content.Number = int64(len(v.Revisions)) + 1
		content.Author, content.SavedAt = actor, now
		v.Revisions = append(v.Revisions, content)
		c.Revision = content.Number
	case "publish":
		if selected == nil {
			return nil, ErrNotFound
		}
		v.Published = selected.Number
		v.Mode = "own"
	case "mode":
		c.Revision = 0
		if c.Mode != "off" && c.Mode != "own" && !(c.Mode == "inherit" && kind == "domains") {
			return nil, ErrWelcomeInvalid
		}
		if c.Mode == "own" && v.Published == 0 {
			return nil, ErrWelcomeInvalid
		}
		v.Mode = c.Mode
	default:
		return nil, ErrWelcomeInvalid
	}
	v.Generation++
	v.Events = append(v.Events, WelcomeEvent{Action: c.Action, Revision: c.Revision, Mode: v.Mode, Actor: actor, At: now})
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var result sql.Result
	if c.Expected == 0 {
		// The scope FK prevents orphan templates and cascades deletion.
		var tenantID string
		var domainID any
		if kind == "tenants" {
			tenantID = id
		} else if kind == "domains" {
			d, e := s.GetDomainByID(ctx, id)
			if e != nil {
				return nil, e
			}
			tenantID = d.TenantID
			domainID = id
		} else {
			return nil, ErrWelcomeInvalid
		}
		result, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO welcome_templates(kind,scope_id,tenant_id,domain_id,generation,snapshot) VALUES(?,?,?,?,?,?) ON CONFLICT(kind,scope_id) DO NOTHING`), kind, id, tenantID, domainID, v.Generation, string(raw))
	} else {
		result, err = s.db.ExecContext(ctx, s.rebind(`UPDATE welcome_templates SET generation=?,snapshot=? WHERE kind=? AND scope_id=? AND generation=?`), v.Generation, string(raw), kind, id, c.Expected)
	}
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrWelcomeConflict
	}
	return v, nil
}

func (s *Store) ListWelcomeTemplates(ctx context.Context, tenantID string) ([]WelcomeTemplate, error) {
	query := `SELECT snapshot FROM welcome_templates`
	var args []any
	if tenantID != "" {
		query += ` WHERE tenant_id=?`
		args = append(args, tenantID)
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WelcomeTemplate{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var v WelcomeTemplate
		if err = json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ImportWelcomeTemplate only fills an absent scope: restoring a backup never
// overwrites a live history or decreases its generation.
func (s *Store) ImportWelcomeTemplate(ctx context.Context, v WelcomeTemplate) error {
	var tenantID string
	var domainID any
	switch v.Kind {
	case "tenants":
		if _, err := s.GetTenantByID(ctx, v.ScopeID); err != nil {
			return err
		}
		tenantID = v.ScopeID
	case "domains":
		d, err := s.GetDomainByID(ctx, v.ScopeID)
		if err != nil {
			return err
		}
		tenantID = d.TenantID
		domainID = d.ID
	default:
		return ErrWelcomeInvalid
	}
	if v.Generation < 1 || v.Generation < int64(len(v.Revisions)) || v.Published < 0 || v.Published > int64(len(v.Revisions)) {
		return ErrWelcomeInvalid
	}
	if v.Mode != "off" && v.Mode != "own" && !(v.Mode == "inherit" && v.Kind == "domains") {
		return ErrWelcomeInvalid
	}
	if v.Mode == "own" && v.Published == 0 {
		return ErrWelcomeInvalid
	}
	for i, r := range v.Revisions {
		if r.Number != int64(i)+1 {
			return fmt.Errorf("welcome revision sequence: %w", ErrWelcomeInvalid)
		}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO welcome_templates(kind,scope_id,tenant_id,domain_id,generation,snapshot) VALUES(?,?,?,?,?,?) ON CONFLICT(kind,scope_id) DO NOTHING`), v.Kind, v.ScopeID, tenantID, domainID, v.Generation, string(raw))
	return err
}
