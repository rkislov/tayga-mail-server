package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestCalendarAttachments(t *testing.T) {
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

	ten, _ := store.CreateTenant(ctx, "t")
	dom, _ := store.CreateDomain(ctx, ten.ID, "ex.com")
	u, err := store.CreateUser(ctx, &storage.User{
		TenantID: ten.ID, DomainID: dom.ID, Email: "u@ex.com", LocalPart: "u",
		AuthSource: "local", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cal, err := store.EnsureCalendar(ctx, u.ID, "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC()
	end := start.Add(time.Hour)
	obj, err := store.UpsertCalendarObject(ctx, &storage.CalendarObject{
		CalendarID: cal.ID, UID: "e1", HrefName: "e1.ics",
		Data: "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n", Size: 10, Component: "VEVENT",
		DTStart: &start, DTEnd: &end, ETag: `"x"`,
	})
	if err != nil {
		t.Fatal(err)
	}
	att, err := store.CreateCalendarAttachment(ctx, &storage.CalendarAttachment{
		CalendarObjectID: obj.ID, UserID: u.ID,
		Filename: "agenda.pdf", ContentType: "application/pdf", Size: 123,
		StoragePath: u.ID + "/" + obj.ID + "/agenda.pdf",
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.ListCalendarAttachments(ctx, obj.ID)
	if err != nil || len(list) != 1 || list[0].ID != att.ID {
		t.Fatalf("%+v err=%v", list, err)
	}
	got, err := store.GetCalendarAttachment(ctx, att.ID)
	if err != nil || got.Filename != "agenda.pdf" {
		t.Fatalf("%+v err=%v", got, err)
	}
	if err := store.DeleteCalendarAttachment(ctx, att.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetCalendarAttachment(ctx, att.ID); err != storage.ErrNotFound {
		t.Fatalf("want not found, got %v", err)
	}
}
