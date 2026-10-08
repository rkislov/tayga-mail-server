package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestCalendarResourcesCRUDAndBusy(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ten, err := store.CreateTenant(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	dom, err := store.CreateDomain(ctx, ten.ID, "ex.com")
	if err != nil {
		t.Fatal(err)
	}

	res, err := store.CreateCalendarResource(ctx, &storage.CalendarResource{
		DomainID: dom.ID, LocalPart: "room-201", DisplayName: "Комната 201",
		Kind: "room", Capacity: 8, AutoAccept: true, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Email != "room-201@ex.com" || res.UserID == "" {
		t.Fatalf("%+v", res)
	}
	got, err := store.GetCalendarResourceByEmail(ctx, "room-201@ex.com")
	if err != nil || got.ID != res.ID {
		t.Fatalf("%+v err=%v", got, err)
	}
	u, err := store.ResolveRecipient(ctx, res.Email)
	if err != nil || u.AuthSource != "resource" {
		t.Fatalf("resolve: %+v err=%v", u, err)
	}

	proj, err := store.CreateCalendarResource(ctx, &storage.CalendarResource{
		DomainID: dom.ID, LocalPart: "projector-1", DisplayName: "Проектор",
		Kind: "equipment", AutoAccept: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.ListCalendarResourcesByDomain(ctx, dom.ID, true)
	if err != nil || len(list) != 2 {
		t.Fatalf("list=%d err=%v", len(list), err)
	}

	start := time.Now().UTC().Truncate(time.Hour)
	end := start.Add(time.Hour)
	cal, err := store.EnsureCalendar(ctx, res.UserID, "default", res.DisplayName)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.UpsertCalendarObject(ctx, &storage.CalendarObject{
		CalendarID: cal.ID, UID: "r1", HrefName: "r1.ics",
		Data: "x", Size: 1, Component: "VEVENT",
		DTStart: &start, DTEnd: &end, ETag: `"e"`,
	})
	if err != nil {
		t.Fatal(err)
	}
	busy, err := store.FreeBusyForUser(ctx, res.UserID, start.Add(-time.Minute), end.Add(time.Minute))
	if err != nil || len(busy) != 1 {
		t.Fatalf("busy=%v err=%v", busy, err)
	}

	proj.DisplayName = "Проектор HD"
	proj.Capacity = 0
	if err := store.UpdateCalendarResource(ctx, proj); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteCalendarResource(ctx, proj.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetCalendarResource(ctx, proj.ID); err != storage.ErrNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
	if _, err := store.GetUserByID(ctx, proj.UserID); err != storage.ErrNotFound {
		t.Fatalf("backing user should be deleted, got %v", err)
	}
}
