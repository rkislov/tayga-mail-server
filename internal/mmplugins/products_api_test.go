package mmplugins

import (
	"context"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func TestOriginalProductScopeAndRoles(t *testing.T) {
	opts := testOptions(t)
	ctx := context.Background()
	tenant, err := opts.Store.CreateTenant(ctx, "Products")
	if err != nil {
		t.Fatal(err)
	}
	var users []*storage.User
	for _, name := range []string{"one.test", "two.test"} {
		domain, err := opts.Store.CreateDomain(ctx, tenant.ID, name)
		if err != nil {
			t.Fatal(err)
		}
		user, err := opts.Store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "owner@" + name, LocalPart: "owner", Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, user)
	}
	opts.TenantID, opts.DomainID = tenant.ID, users[0].DomainID
	opts.Scope = "domain:" + tenant.ID + ":" + opts.DomainID
	makeHost := func(options Options, id string) *hostAPI {
		return &hostAPI{unsupportedAPI: &unsupportedAPI{log: options.Logger, pluginID: id}, runtime: &Runtime{options: options}, manifest: &model.Manifest{Id: id, Name: id}}
	}
	host := makeHost(opts, "playbooks")
	group, err := host.RegisterPropertyGroup("playbooks")
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := makeHost(opts, "playbooks").RegisterPropertyGroup("playbooks")
	if err != nil || repeated.ID != group.ID {
		t.Fatal("group did not survive host recreation", err)
	}
	otherOptions := opts
	otherOptions.DomainID = users[1].DomainID
	otherOptions.Scope = "domain:" + tenant.ID + ":" + otherOptions.DomainID
	for _, other := range []*hostAPI{makeHost(opts, "focalboard"), makeHost(otherOptions, "playbooks")} {
		if _, err := other.GetPropertyGroup("playbooks"); err == nil {
			t.Fatal("property group leaked across scope/plugin")
		}
		otherGroup, err := other.RegisterPropertyGroup("playbooks")
		if err != nil || otherGroup.ID == group.ID {
			t.Fatal("group identity collision", err)
		}
	}
	if !host.RolesGrantPermission([]string{model.PlaybookMemberRoleId}, model.PermissionPrivatePlaybookView.Id) {
		t.Fatal("upstream member view permission missing")
	}
	if host.RolesGrantPermission([]string{model.PlaybookMemberRoleId, "unknown"}, model.PermissionManageSystem.Id) {
		t.Fatal("member got server administration")
	}
	if host.HasPermissionToTeam(chat.ID(users[1].ID), chat.ID(tenant.ID), model.PermissionRunCreate) {
		t.Fatal("foreign domain got run permission")
	}
	if _, err := host.CreateTeamMember(chat.ID(tenant.ID), chat.ID(users[1].ID)); err == nil {
		t.Fatal("foreign domain membership exposed")
	}
	if _, err := host.CreateChannel(&model.Channel{TeamId: chat.ID(tenant.ID), CreatorId: chat.ID(users[1].ID), Name: "foreign", DisplayName: "Foreign", Type: model.ChannelTypePrivate}); err == nil {
		t.Fatal("foreign creator accepted")
	}
	channel, appErr := host.CreateChannel(&model.Channel{TeamId: chat.ID(tenant.ID), Name: "procedure", DisplayName: "Procedure", Type: model.ChannelTypePrivate})
	if appErr != nil {
		t.Fatal(appErr)
	}
	if _, appErr := host.AddUserToChannel(channel.Id, chat.ID(users[0].ID), channel.CreatorId); appErr != nil {
		t.Fatal(appErr)
	}
	if _, appErr := host.GetChannelMember(channel.Id, chat.ID(users[0].ID)); appErr != nil {
		t.Fatal(appErr)
	}
	if _, appErr := host.AddUserToChannel(channel.Id, chat.ID(users[1].ID), channel.CreatorId); appErr == nil {
		t.Fatal("foreign user added")
	}
	if _, appErr := host.AddUserToChannel(channel.Id, chat.ID(users[0].ID), chat.ID(users[1].ID)); appErr == nil {
		t.Fatal("foreign actor accepted")
	}
}
