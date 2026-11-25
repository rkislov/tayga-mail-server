package chat

import (
	"path/filepath"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestMembershipHistoryTracksJoinLeaveRejoin(t *testing.T) {
	ctx := t.Context()
	cfg := config.Default()
	cfg.Storage.SQLite.Path = filepath.Join(t.TempDir(), "history.db")
	st, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	service := New(st)
	tenant, err := st.CreateTenant(ctx, "History")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := st.CreateDomain(ctx, tenant.ID, "history.test")
	if err != nil {
		t.Fatal(err)
	}
	users := []*storage.User{}
	for _, name := range []string{"owner", "member", "other"} {
		u, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: name + "@history.test", LocalPart: name, Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, u)
	}
	room, err := service.CreateChannel(ctx, users[0], "history-room", "History", "P")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err = service.AddMember(ctx, users[0], room.ID, ID(users[1].ID)); err != nil {
			t.Fatal(err)
		}
	}
	count := func(active bool) int {
		q := `SELECT count(*) FROM chat_member_history WHERE channel_id=? AND user_id=?`
		if active {
			q += ` AND left_at IS NULL`
		}
		var n int
		if err := st.DB().QueryRowContext(ctx, q, room.ID, users[1].ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count(false) != 1 || count(true) != 1 {
		t.Fatal("duplicate membership join events")
	}
	if err = service.RemoveMember(ctx, users[2], room.ID, ID(users[1].ID)); err == nil {
		t.Fatal("outsider removed membership")
	}
	if err = service.RemoveMember(ctx, users[0], room.ID, ID(users[0].ID)); err == nil {
		t.Fatal("creator removed without transferring ownership")
	}
	if err = service.RemoveMember(ctx, users[1], room.ID, ID(users[1].ID)); err != nil {
		t.Fatal(err)
	}
	if count(false) != 1 || count(true) != 0 {
		t.Fatal("leave was not recorded")
	}
	if err = service.AddMember(ctx, users[0], room.ID, ID(users[1].ID)); err != nil {
		t.Fatal(err)
	}
	if count(false) != 2 || count(true) != 1 {
		t.Fatal("rejoin history lost")
	}
	public, err := service.CreateChannel(ctx, users[0], "public-history", "Public", "O")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err = service.JoinPublicChannel(ctx, users[1], public.ID); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err = st.DB().QueryRowContext(ctx, `SELECT count(*) FROM chat_member_history WHERE channel_id=? AND user_id=?`, public.ID, users[1].ID).Scan(&n); err != nil || n != 1 {
		t.Fatal("public join was not atomic/idempotent", n, err)
	}
}
