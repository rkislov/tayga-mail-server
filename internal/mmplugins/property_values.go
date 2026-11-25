package mmplugins

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/storage"
)

func (a *hostAPI) GetPropertyValue(groupID, id string) (*model.PropertyValue, error) {
	db, err := a.propertyDB(groupID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	var raw []byte
	err = db.QueryRowContext(ctx, `SELECT data FROM TMS_PropertyValues WHERE group_id=$1 AND id=$2`, groupID, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var v model.PropertyValue
	err = json.Unmarshal(raw, &v)
	return &v, err
}
func (a *hostAPI) GetPropertyValues(groupID string, ids []string) ([]*model.PropertyValue, error) {
	if len(ids) > 1000 {
		return nil, ErrInvalid
	}
	out := []*model.PropertyValue{}
	for _, id := range ids {
		v, err := a.GetPropertyValue(groupID, id)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
func (a *hostAPI) CreatePropertyValue(v *model.PropertyValue) (*model.PropertyValue, error) {
	if v == nil {
		return nil, ErrInvalid
	}
	copy := *v
	copy.ID = ""
	copy.CreateAt = 0
	copy.UpdateAt = 0
	copy.DeleteAt = 0
	copy.PreSave()
	return a.writePropertyValue(&copy, false)
}
func (a *hostAPI) writePropertyValue(v *model.PropertyValue, update bool) (*model.PropertyValue, error) {
	if err := v.IsValid(); err != nil {
		return nil, err
	}
	f, err := a.GetPropertyField(v.GroupID, v.FieldID)
	if err != nil {
		return nil, err
	}
	if f.DeleteAt != 0 {
		return nil, storage.ErrNotFound
	}
	db, err := a.propertyDB(v.GroupID)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	q := `INSERT INTO TMS_PropertyValues(id,group_id,data) VALUES($1,$2,$3)`
	if update {
		q = `UPDATE TMS_PropertyValues SET data=$3 WHERE id=$1 AND group_id=$2`
	}
	_, err = db.ExecContext(ctx, q, v.ID, v.GroupID, string(raw))
	return v, err
}
func (a *hostAPI) UpdatePropertyValue(groupID string, v *model.PropertyValue) (*model.PropertyValue, error) {
	if v == nil || v.GroupID != groupID {
		return nil, ErrInvalid
	}
	old, err := a.GetPropertyValue(groupID, v.ID)
	if err != nil {
		return nil, err
	}
	if old.FieldID != v.FieldID || old.TargetID != v.TargetID || old.TargetType != v.TargetType {
		return nil, ErrInvalid
	}
	copy := *v
	copy.CreateAt = old.CreateAt
	copy.UpdateAt = time.Now().UnixMilli()
	if copy.UpdateAt <= old.UpdateAt {
		copy.UpdateAt = old.UpdateAt + 1
	}
	return a.writePropertyValue(&copy, true)
}
func (a *hostAPI) DeletePropertyValue(groupID, id string) error {
	v, err := a.GetPropertyValue(groupID, id)
	if err != nil {
		return err
	}
	v.DeleteAt = time.Now().UnixMilli()
	_, err = a.UpdatePropertyValue(groupID, v)
	return err
}
func (a *hostAPI) SearchPropertyValues(groupID string, o model.PropertyValueSearchOpts) ([]*model.PropertyValue, error) {
	// Playbooks 2.9.x uses the pre-UpdateAt cursor wire format. Resolve
	// its last record inside this property group instead of trusting a timestamp
	// from a different SDK generation. New SDK cursors pass through unchanged.
	if o.SinceUpdateAt > 0 && o.Cursor.UpdateAt == 0 && o.Cursor.CreateAt > 0 && o.Cursor.PropertyValueID != "" {
		last, err := a.GetPropertyValue(groupID, o.Cursor.PropertyValueID)
		if err != nil {
			return nil, err
		}
		o.Cursor.UpdateAt = last.UpdateAt
		o.Cursor.CreateAt = 0
	}
	if err := o.IsValid(); err != nil {
		return nil, err
	}
	if o.GroupID != "" && o.GroupID != groupID {
		return nil, ErrInvalid
	}
	db, err := a.propertyDB(groupID)
	if err != nil {
		return nil, err
	}
	q := `SELECT data FROM TMS_PropertyValues WHERE group_id=$1`
	args := []any{groupID}
	add := func(expr string, value any) { args = append(args, value); q += " AND " + fmt.Sprintf(expr, len(args)) }
	if !o.IncludeDeleted && o.SinceUpdateAt == 0 {
		q += ` AND COALESCE((data->>'delete_at')::bigint,0)=0`
	}
	if o.TargetType != "" {
		add(`data->>'target_type'=$%d`, o.TargetType)
	}
	if o.FieldID != "" {
		add(`data->>'field_id'=$%d`, o.FieldID)
	}
	if len(o.Value) > 0 {
		if !json.Valid(o.Value) {
			return nil, ErrInvalid
		}
		add(`data->'value'=$%d::jsonb`, string(o.Value))
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
		q += " AND " + key + " IN (" + strings.Join(parts, ",") + ")"
	}
	in("data->>'target_id'", o.TargetIDs)
	in("id", o.IDs)
	order := "create_at"
	cursor := o.Cursor.CreateAt
	if o.SinceUpdateAt > 0 {
		order = "update_at"
		cursor = o.Cursor.UpdateAt
		add(`(data->>'update_at')::bigint>=$%d`, o.SinceUpdateAt)
	}
	if !o.Cursor.IsEmpty() {
		args = append(args, cursor, o.Cursor.PropertyValueID)
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
	ctx, cancel := sqlTimeout()
	defer cancel()
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*model.PropertyValue{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var v model.PropertyValue
		if err = json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		out = append(out, &v)
	}
	return out, rows.Err()
}

func (a *hostAPI) UpsertPropertyValue(value *model.PropertyValue) (*model.PropertyValue, error) {
	if value == nil {
		return nil, ErrInvalid
	}
	v := *value
	v.ID = ""
	v.CreateAt = 0
	v.UpdateAt = 0
	v.DeleteAt = 0
	v.PreSave()
	if err := v.IsValid(); err != nil {
		return nil, err
	}
	f, err := a.GetPropertyField(v.GroupID, v.FieldID)
	if err != nil {
		return nil, err
	}
	if f.DeleteAt != 0 {
		return nil, storage.ErrNotFound
	}
	db, err := a.propertyDB(v.GroupID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	key := v.GroupID + ":" + v.FieldID + ":" + v.TargetType + ":" + v.TargetID
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		return nil, err
	}
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT data FROM TMS_PropertyValues WHERE group_id=$1 AND data->>'field_id'=$2 AND data->>'target_type'=$3 AND data->>'target_id'=$4 FOR UPDATE`, v.GroupID, v.FieldID, v.TargetType, v.TargetID).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		var old model.PropertyValue
		if err = json.Unmarshal(raw, &old); err != nil {
			return nil, err
		}
		v.ID = old.ID
		v.CreateAt = old.CreateAt
		if v.UpdateAt <= old.UpdateAt {
			v.UpdateAt = old.UpdateAt + 1
		}
	}
	raw, err = json.Marshal(v)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO TMS_PropertyValues(id,group_id,data) VALUES($1,$2,$3) ON CONFLICT(id) DO UPDATE SET data=excluded.data WHERE TMS_PropertyValues.group_id=excluded.group_id`, v.ID, v.GroupID, string(raw))
	if err != nil {
		return nil, err
	}
	err = tx.Commit()
	return &v, err
}
