package mmplugins

import (
	"context"
	"database/sql"

	"github.com/tayga/tms/internal/chat"
)

const productCoreSchema = `CREATE TABLE IF NOT EXISTS Users(Id VARCHAR(26) PRIMARY KEY, Username TEXT, Email TEXT, FirstName TEXT, LastName TEXT, Nickname TEXT, DeleteAt BIGINT DEFAULT 0, Roles TEXT);
CREATE TABLE IF NOT EXISTS Teams(Id VARCHAR(26) PRIMARY KEY, Name TEXT, DisplayName TEXT, DeleteAt BIGINT DEFAULT 0, Type TEXT, SchemeId VARCHAR(26));
CREATE TABLE IF NOT EXISTS Schemes(Id VARCHAR(26) PRIMARY KEY,DefaultPlaybookAdminRole TEXT,DefaultPlaybookMemberRole TEXT,DefaultRunAdminRole TEXT,DefaultRunMemberRole TEXT);
CREATE TABLE IF NOT EXISTS TeamMembers(TeamId VARCHAR(26),UserId VARCHAR(26),Roles TEXT,DeleteAt BIGINT DEFAULT 0);
CREATE TABLE IF NOT EXISTS Channels(Id VARCHAR(26) PRIMARY KEY,TeamId VARCHAR(26),CreatorId VARCHAR(26),Name TEXT,DisplayName TEXT,CreateAt BIGINT DEFAULT 0,UpdateAt BIGINT DEFAULT 0,DeleteAt BIGINT DEFAULT 0,Type TEXT);
CREATE TABLE IF NOT EXISTS ChannelMembers(ChannelId VARCHAR(26),UserId VARCHAR(26),Roles TEXT);
CREATE TABLE IF NOT EXISTS PluginKeyValueStore(PluginId VARCHAR(190),PKey VARCHAR(255),PValue BYTEA,ExpireAt BIGINT DEFAULT 0,PRIMARY KEY(PluginId,PKey));
CREATE TABLE IF NOT EXISTS Posts(Id VARCHAR(26) PRIMARY KEY,ChannelId VARCHAR(26),UserId VARCHAR(26),RootId VARCHAR(26),CreateAt BIGINT DEFAULT 0,UpdateAt BIGINT DEFAULT 0,DeleteAt BIGINT DEFAULT 0,Message TEXT,Type TEXT,Props JSONB DEFAULT '{}');
CREATE TABLE IF NOT EXISTS FileInfo(Id VARCHAR(26) PRIMARY KEY,CreatorId VARCHAR(26),PostId TEXT DEFAULT '',CreateAt BIGINT DEFAULT 0,UpdateAt BIGINT DEFAULT 0,DeleteAt BIGINT DEFAULT 0,Path TEXT,ThumbnailPath TEXT DEFAULT '',PreviewPath TEXT DEFAULT '',Name TEXT,Extension TEXT,Size BIGINT DEFAULT 0,MimeType TEXT,Width INTEGER DEFAULT 0,Height INTEGER DEFAULT 0,HasPreviewImage BOOLEAN DEFAULT false,MiniPreview BYTEA,Content TEXT,RemoteId TEXT,Archived BOOLEAN DEFAULT false);
CREATE TABLE IF NOT EXISTS ChannelMemberHistory(ChannelId VARCHAR(26),UserId VARCHAR(26),JoinTime BIGINT NOT NULL,LeaveTime BIGINT);
CREATE TABLE IF NOT EXISTS Bots(UserId VARCHAR(26),Username TEXT,OwnerId TEXT,DeleteAt BIGINT DEFAULT 0);`

// The compatibility mirror contains this domain's active principals and channel
// memberships only. It is not an authentication database and contains no login
// secrets, mail, external connections, or data from sibling domains.
func prepareProductMetadata(ctx context.Context, db *sql.DB, options Options) error {
	if options.TenantID == "" || options.DomainID == "" {
		return ErrInvalid
	}
	if _, err := db.ExecContext(ctx, productCoreSchema); err != nil {
		return err
	}
	return syncProductMetadata(ctx, db, options)
}

func syncProductMetadata(ctx context.Context, db *sql.DB, options Options) error {
	users, err := options.Store.ListUsersByDomain(ctx, options.DomainID)
	if err != nil {
		return err
	}
	tenants, err := options.Store.ListTenants(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(780077335)`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM ChannelMemberHistory;DELETE FROM Posts;DELETE FROM ChannelMembers;DELETE FROM Channels;DELETE FROM TeamMembers;DELETE FROM Users;DELETE FROM Teams;DELETE FROM Bots;`); err != nil {
		return err
	}
	teamID := chat.ID(options.TenantID)
	found := false
	for _, tenant := range tenants {
		if tenant.ID == options.TenantID {
			found = true
			if _, err = tx.ExecContext(ctx, `INSERT INTO Teams(Id,Name,DisplayName,Type) VALUES($1,$2,$3,'I')`, teamID, "tms-"+teamID, tenant.Name); err != nil {
				return err
			}
		}
	}
	if !found {
		return ErrInvalid
	}
	visible := map[string]string{}
	for _, user := range users {
		if !user.Enabled || user.TenantID != options.TenantID {
			continue
		}
		model := modelUser(user)
		visible[user.ID] = model.Id
		if _, err = tx.ExecContext(ctx, `INSERT INTO Users(Id,Username,Email,FirstName,LastName,Nickname,Roles) VALUES($1,$2,$3,$4,$5,$6,$7)`, model.Id, model.Username, model.Email, model.FirstName, model.LastName, model.Nickname, model.Roles); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO TeamMembers(TeamId,UserId,Roles) VALUES($1,$2,'team_user')`, teamID, model.Id); err != nil {
			return err
		}
		if model.IsBot {
			if _, err = tx.ExecContext(ctx, `INSERT INTO Bots(UserId,Username,OwnerId) VALUES($1,$2,'')`, model.Id, model.Username); err != nil {
				return err
			}
		}
	}
	r := &Runtime{options: options}
	rows, err := options.Store.DB().QueryContext(ctx, r.bind(`SELECT c.id,c.creator_id,c.name,c.display_name,c.kind,c.created_at FROM chat_channels c JOIN users u ON c.creator_id=u.id WHERE c.tenant_id=? AND u.domain_id=?`), options.TenantID, options.DomainID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type sourceChannel struct {
		id, creator, name, display, kind string
		created                          int64
	}
	channels := []sourceChannel{}
	for rows.Next() {
		var id, creator, name, display, kind string
		var created int64
		if err = rows.Scan(&id, &creator, &name, &display, &kind, &created); err != nil {
			return err
		}
		channels = append(channels, sourceChannel{id, creator, name, display, kind, created})
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, channel := range channels {
		id, creator, name, display, kind, created := channel.id, channel.creator, channel.name, channel.display, channel.kind, channel.created

		owner, ok := visible[creator]
		if !ok {
			continue
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO Channels(Id,TeamId,CreatorId,Name,DisplayName,Type,CreateAt,UpdateAt) VALUES($1,$2,$3,$4,$5,$6,$7,$7)`, id, teamID, owner, name, display, kind, created); err != nil {
			return err
		}
		members, err := options.Store.DB().QueryContext(ctx, r.bind(`SELECT user_id FROM chat_members WHERE channel_id=?`), id)
		if err != nil {
			return err
		}
		for members.Next() {
			var userID string
			if err = members.Scan(&userID); err != nil {
				members.Close()
				return err
			}
			mid, ok := visible[userID]
			if !ok {
				continue
			}
			roles := "channel_user"
			if userID == creator {
				roles += " channel_admin"
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO ChannelMembers(ChannelId,UserId,Roles) VALUES($1,$2,$3)`, id, mid, roles); err != nil {
				members.Close()
				return err
			}
		}
		err = members.Err()
		members.Close()
		if err != nil {
			return err
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	history, err := options.Store.DB().QueryContext(ctx, r.bind(`SELECT h.channel_id,h.user_id,h.joined_at,h.left_at FROM chat_member_history h JOIN chat_channels c ON c.id=h.channel_id JOIN users owner ON owner.id=c.creator_id JOIN users u ON u.id=h.user_id WHERE c.tenant_id=? AND owner.domain_id=? AND u.domain_id=?`), options.TenantID, options.DomainID, options.DomainID)
	if err != nil {
		return err
	}
	for history.Next() {
		var channelID, userID string
		var joined int64
		var left sql.NullInt64
		if err = history.Scan(&channelID, &userID, &joined, &left); err != nil {
			history.Close()
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO ChannelMemberHistory(ChannelId,UserId,JoinTime,LeaveTime) VALUES($1,$2,$3,$4)`, channelID, chat.ID(userID), joined, left); err != nil {
			history.Close()
			return err
		}
	}
	err = history.Err()
	history.Close()
	if err != nil {
		return err
	}
	posts, err := options.Store.DB().QueryContext(ctx, r.bind(`SELECT p.id,p.channel_id,p.user_id,p.root_id,p.message,p.post_type,p.props,p.created_at FROM chat_posts p JOIN chat_channels c ON c.id=p.channel_id JOIN users owner ON owner.id=c.creator_id JOIN users author ON author.id=p.user_id WHERE c.tenant_id=? AND owner.domain_id=? AND author.domain_id=?`), options.TenantID, options.DomainID, options.DomainID)
	if err != nil {
		return err
	}
	for posts.Next() {
		var id, channelID, userID, rootID, message, postType, props string
		var created int64
		if err = posts.Scan(&id, &channelID, &userID, &rootID, &message, &postType, &props, &created); err != nil {
			posts.Close()
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO Posts(Id,ChannelId,UserId,RootId,Message,Type,Props,CreateAt,UpdateAt) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8)`, id, channelID, chat.ID(userID), rootID, message, postType, props, created); err != nil {
			posts.Close()
			return err
		}
	}
	err = posts.Err()
	posts.Close()
	if err != nil {
		return err
	}

	return tx.Commit()
}
