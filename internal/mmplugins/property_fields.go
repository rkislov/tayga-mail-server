package mmplugins

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/storage"
)

// Property data lives in the plugin's dedicated SQL database. The host never
// forwards queries to TMS mail/account storage.
func (a *hostAPI) propertyDB(groupID string) (*sql.DB, error) {
	if !model.IsValidId(groupID) {
		return nil, ErrInvalid
	}
	marker, err := a.readKV("property.group.id:" + groupID)
	if err != nil {
		return nil, err
	}
	if len(marker) == 0 {
		return nil, storage.ErrNotFound
	}
	if a.runtime.sql == nil || a.runtime.options.SQLDriverName != "postgres" {
		return nil, errors.New("property storage requires dedicated PostgreSQL")
	}
	a.runtime.sql.mu.Lock()
	db := a.runtime.sql.pools[a.manifest.Id]
	a.runtime.sql.mu.Unlock()
	if db == nil {
		return nil, errors.New("plugin database unavailable")
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS TMS_PropertyFields(id VARCHAR(26) PRIMARY KEY,group_id VARCHAR(26) NOT NULL,data JSONB NOT NULL);
 CREATE INDEX IF NOT EXISTS tms_propertyfields_group ON TMS_PropertyFields(group_id);
 CREATE TABLE IF NOT EXISTS TMS_PropertyValues(id VARCHAR(26) PRIMARY KEY,group_id VARCHAR(26) NOT NULL,data JSONB NOT NULL);
 CREATE INDEX IF NOT EXISTS tms_propertyvalues_group ON TMS_PropertyValues(group_id);
 CREATE UNIQUE INDEX IF NOT EXISTS tms_propertyvalues_identity ON TMS_PropertyValues(group_id,(data->>'field_id'),(data->>'target_type'),(data->>'target_id'));`)
	return db, err
}
func (a *hostAPI) CreatePropertyField(field *model.PropertyField) (*model.PropertyField, error) {
	if field == nil || (field.LinkedFieldID != nil && *field.LinkedFieldID != "") {
		return nil, ErrInvalid
	}
	copy := *field
	copy.ID = ""
	copy.CreateAt = 0
	copy.UpdateAt = 0
	copy.DeleteAt = 0
	copy.PreSave()
	if err := copy.IsValid(); err != nil {
		return nil, err
	}
	db, err := a.propertyDB(copy.GroupID)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(copy)
	if err != nil {
		return nil, err
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	_, err = db.ExecContext(ctx, `INSERT INTO TMS_PropertyFields(id,group_id,data) VALUES($1,$2,$3)`, copy.ID, copy.GroupID, string(raw))
	return &copy, err
}
func (a *hostAPI) GetPropertyField(groupID, id string) (*model.PropertyField, error) {
	db, err := a.propertyDB(groupID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	var raw []byte
	err = db.QueryRowContext(ctx, `SELECT data FROM TMS_PropertyFields WHERE group_id=$1 AND id=$2`, groupID, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var field model.PropertyField
	err = json.Unmarshal(raw, &field)
	return &field, err
}
func (a *hostAPI) GetPropertyFields(groupID string, ids []string) ([]*model.PropertyField, error) {
	if len(ids) > 1000 {
		return nil, ErrInvalid
	}
	out := []*model.PropertyField{}
	for _, id := range ids {
		f, err := a.GetPropertyField(groupID, id)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}
func (a *hostAPI) UpdatePropertyField(groupID string, field *model.PropertyField) (*model.PropertyField, error) {
	if field == nil || groupID != field.GroupID || (field.LinkedFieldID != nil && *field.LinkedFieldID != "") {
		return nil, ErrInvalid
	}
	old, err := a.GetPropertyField(groupID, field.ID)
	if err != nil {
		return nil, err
	}
	copy := *field
	copy.CreateAt = old.CreateAt
	copy.UpdateAt = time.Now().UnixMilli()
	if copy.UpdateAt <= old.UpdateAt {
		copy.UpdateAt = old.UpdateAt + 1
	}
	if err = copy.IsValid(); err != nil {
		return nil, err
	}
	db, err := a.propertyDB(groupID)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(copy)
	if err != nil {
		return nil, err
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	_, err = db.ExecContext(ctx, `UPDATE TMS_PropertyFields SET data=$3 WHERE group_id=$1 AND id=$2`, groupID, copy.ID, string(raw))
	return &copy, err
}
func (a *hostAPI) DeletePropertyField(groupID, id string) error {
	field, err := a.GetPropertyField(groupID, id)
	if err != nil {
		return err
	}
	field.DeleteAt = time.Now().UnixMilli()
	_, err = a.UpdatePropertyField(groupID, field)
	return err
}
func (a *hostAPI) SearchPropertyFields(groupID string, o model.PropertyFieldSearchOpts) ([]*model.PropertyField, error) {
	// Playbooks 2.9.x uses the pre-UpdateAt cursor wire format. Resolve
	// its last record inside this property group instead of trusting a timestamp
	// from a different SDK generation. New SDK cursors pass through unchanged.
	if o.SinceUpdateAt > 0 && o.Cursor.UpdateAt == 0 && o.Cursor.CreateAt > 0 && o.Cursor.PropertyFieldID != "" {
		last, err := a.GetPropertyField(groupID, o.Cursor.PropertyFieldID)
		if err != nil {
			return nil, err
		}
		o.Cursor.UpdateAt = last.UpdateAt
		o.Cursor.CreateAt = 0
	}
	if err := o.IsValid(); err != nil {
		return nil, err
	}
	if (o.GroupID != "" && o.GroupID != groupID) || o.ChannelID != "" || o.TeamID != "" {
		return nil, ErrInvalid
	}
	db, err := a.propertyDB(groupID)
	if err != nil {
		return nil, err
	}
	q := `SELECT data FROM TMS_PropertyFields WHERE group_id=$1`
	args := []any{groupID}
	add := func(expr string, value any) { args = append(args, value); q += " AND " + fmt.Sprintf(expr, len(args)) }
	if !o.IncludeDeleted && o.SinceUpdateAt == 0 {
		q += ` AND COALESCE((data->>'delete_at')::bigint,0)=0`
	}
	if o.TargetType != "" {
		add(`data->>'target_type'=$%d`, o.TargetType)
	}
	if o.LinkedFieldID != "" {
		add(`data->>'linked_field_id'=$%d`, o.LinkedFieldID)
	}
	if o.ObjectType != "" {
		add(`data->>'object_type'=$%d`, o.ObjectType)
	}
	in := func(key string, values []string) {
		if len(values) == 0 {
			return
		}
		parts := []string{}
		for _, v := range values {
			args = append(args, v)
			parts = append(parts, fmt.Sprintf("$%d", len(args)))
		}
		q += " AND data->>'" + key + "' IN (" + strings.Join(parts, ",") + ")"
	}
	in("target_id", o.TargetIDs)
	in("object_type", o.ObjectTypes)
	order := "create_at"
	cursor := o.Cursor.CreateAt
	if o.SinceUpdateAt > 0 {
		order = "update_at"
		cursor = o.Cursor.UpdateAt
		add(`(data->>'update_at')::bigint>=$%d`, o.SinceUpdateAt)
	}
	if !o.Cursor.IsEmpty() {
		args = append(args, cursor, o.Cursor.PropertyFieldID)
		q += fmt.Sprintf(` AND ((data->>'%s')::bigint,id)>($%d,$%d)`, order, len(args)-1, len(args))
	}
	limit := o.PerPage
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 || len(args) > 2000 {
		return nil, ErrInvalid
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY (data->>'%s')::bigint,id LIMIT $%d`, order, len(args))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*model.PropertyField{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var f model.PropertyField
		if err = json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		out = append(out, &f)
	}
	return out, rows.Err()
}

func (a *hostAPI) CountPropertyFields(groupID string, includeDeleted bool) (int64, error) {
	return a.countPropertyFields(groupID, "", "", includeDeleted)
}
func (a *hostAPI) CountPropertyFieldsForTarget(groupID, targetType, targetID string, includeDeleted bool) (int64, error) {
	return a.countPropertyFields(groupID, targetType, targetID, includeDeleted)
}
func (a *hostAPI) countPropertyFields(groupID, targetType, targetID string, includeDeleted bool) (int64, error) {
	db, err := a.propertyDB(groupID)
	if err != nil {
		return 0, err
	}
	q := `SELECT count(*) FROM TMS_PropertyFields WHERE group_id=$1`
	args := []any{groupID}
	if !includeDeleted {
		q += ` AND COALESCE((data->>'delete_at')::bigint,0)=0`
	}
	if targetType != "" {
		q += ` AND data->>'target_type'=$2 AND data->>'target_id'=$3`
		args = append(args, targetType, targetID)
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	var n int64
	err = db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}
