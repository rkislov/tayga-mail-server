package mmplugins

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func (a *hostAPI) GetClusterNodeID() string {
	return chat.ID("local-plugin-node:" + a.runtime.options.Scope + ":" + a.GetDiagnosticId())
}

// The current host is single-node. Broadcasts target other nodes, so an empty
// target has no recipients; it is not looped back into the publishing plugin.
func (a *hostAPI) PublishPluginClusterEvent(event model.PluginClusterEvent, opts model.PluginClusterEventSendOptions) error {
	if opts.TargetId == "" {
		return nil
	}
	if opts.TargetId != a.GetClusterNodeID() {
		return errors.New("plugin cluster target is unavailable on this single-node host")
	}
	return fmt.Errorf("self-targeted plugin cluster delivery is not supported")
}

func (a *hostAPI) GetFileInfo(id string) (*model.FileInfo, *model.AppError) {
	if a.runtime.sql == nil {
		return a.unsupportedAPI.GetFileInfo(id)
	}
	a.runtime.sql.mu.Lock()
	db := a.runtime.sql.pools[a.manifest.Id]
	a.runtime.sql.mu.Unlock()
	if db == nil {
		return nil, appError(storage.ErrNotFound)
	}
	// This is the plugin's own SQL database, not the mail/chat database. Original
	// Boards writes its file metadata through the SDK SQL connection.
	var file model.FileInfo
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := db.QueryRowContext(ctx, `SELECT Id,CreatorId,COALESCE(PostId,''),Name,Extension,Size,MimeType,Width,Height,HasPreviewImage,Path FROM FileInfo WHERE Id=$1 AND DeleteAt=0`, id).Scan(&file.Id, &file.CreatorId, &file.PostId, &file.Name, &file.Extension, &file.Size, &file.MimeType, &file.Width, &file.Height, &file.HasPreviewImage, &file.Path)
	if errors.Is(err, sql.ErrNoRows) {
		err = storage.ErrNotFound
	}
	if err != nil {
		return nil, appError(err)
	}
	return &file, nil
}

// RolesGrantPermission evaluates the upstream built-in role definitions. It does
// not assign roles: caller identity and team access remain separate checks.
func (a *hostAPI) RolesGrantPermission(names []string, permissionID string) bool {
	roles := model.MakeDefaultRoles()
	for _, name := range names {
		role := roles[name]
		if role == nil {
			continue
		}
		for _, permission := range role.Permissions {
			if permission == permissionID {
				return true
			}
		}
	}
	return false
}

// Audit records intentionally exclude sessions, payloads, prior/result objects
// and free-form errors; plugin records must not leak message text or credentials.
func (a *hostAPI) LogAuditRec(rec *model.AuditRecord) {
	if rec == nil || !auditLabel.MatchString(rec.EventName) {
		return
	}
	actor := ""
	if rec.Actor.UserId != "" {
		if _, err := a.user(rec.Actor.UserId); err != nil {
			return
		}
		actor = rec.Actor.UserId
	}
	status := "unknown"
	switch rec.Status {
	case "success", "fail", "failed", "attempt":
		status = rec.Status
	}
	a.runtime.options.Logger.Info("plugin audit", "plugin_id", a.manifest.Id, "scope", a.runtime.options.Scope, "event", rec.EventName, "actor_id", actor, "status", status)
}

var auditLabel = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)
