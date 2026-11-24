package mmplugins

import (
	"encoding/json"
	"regexp"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

var propertyGroupName = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{0,63}$`)

// Property groups belong to the requesting plugin in its runtime scope. This
// does not register global server groups or grant access to another plugin.
func (a *hostAPI) GetPropertyGroup(name string) (*model.PropertyGroup, error) {
	if !propertyGroupName.MatchString(name) {
		return nil, ErrInvalid
	}
	raw, e := a.KVGet("property.group:" + name)
	if e != nil {
		return nil, e
	}
	if len(raw) == 0 {
		return nil, storage.ErrNotFound
	}
	var group model.PropertyGroup
	if e := json.Unmarshal(raw, &group); e != nil {
		return nil, e
	}
	if e := a.KVSet("property.group.id:"+group.ID, []byte(group.Name)); e != nil {
		return nil, e
	}
	return &group, nil
}
func (a *hostAPI) RegisterPropertyGroup(name string) (*model.PropertyGroup, error) {
	if !propertyGroupName.MatchString(name) {
		return nil, ErrInvalid
	}
	if group, e := a.GetPropertyGroup(name); e == nil {
		return group, nil
	} else if e != storage.ErrNotFound {
		return nil, e
	}
	group := model.PropertyGroup{ID: chat.ID(a.kvPrefix() + "property.group:" + name), Name: name}
	group.PreSave()
	if e := group.IsValid(); e != nil {
		return nil, e
	}
	raw, e := json.Marshal(group)
	if e != nil {
		return nil, e
	}
	_, appErr := a.KVCompareAndSet("property.group:"+name, nil, raw)
	if appErr != nil {
		return nil, appErr
	}
	return a.GetPropertyGroup(name)
}
