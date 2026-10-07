package smtp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestMailtoAddress(t *testing.T) {
	if got := mailtoAddress("mailto:dmarc@example.com!50m"); got != "dmarc@example.com" {
		t.Fatal(got)
	}
	if got := mailtoAddress("https://x"); got != "" {
		t.Fatal(got)
	}
}

func TestDMARCAggAndXML(t *testing.T) {
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

	rep := newDMARCReporter(store, "Tayga", "dmarc@mail.test", "mail.test", time.Hour, nil)
	rep.Record(&dmarcEvent{
		Domain: "example.com", HeaderFrom: "example.com", SourceIP: "1.2.3.4",
		EnvelopeDomain: "example.com", SPFResult: "pass", DKIMResult: "pass",
		Disposition: "none", Policy: "none",
		RUA: []string{"mailto:rua@example.com"},
	})
	rep.flushBuffer(ctx)

	// Force into yesterday so SendDue picks it up.
	day := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	rows, err := store.ListDMARCAggByDay(ctx, time.Now().UTC().Format("2006-01-02"))
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	row := rows[0]
	row.Day = day
	row.ID = storage.NewID()
	_ = store.DeleteDMARCAggByDay(ctx, time.Now().UTC().Format("2006-01-02"))
	if err := store.UpsertDMARCAgg(ctx, row); err != nil {
		t.Fatal(err)
	}

	xmlBody, err := buildAggregateXML("Tayga", "dmarc@mail.test", "mail.test", "example.com", 1, 2, []*storage.DMARCAggRow{row})
	if err != nil {
		t.Fatal(err)
	}
	s := string(xmlBody)
	if !strings.Contains(s, "<feedback>") || !strings.Contains(s, "<source_ip>1.2.3.4</source_ip>") {
		t.Fatalf("%s", s)
	}
	gz, err := gzipBytes(xmlBody)
	if err != nil || len(gz) == 0 {
		t.Fatal(err)
	}
	msg, err := buildReportMessage("dmarc@mail.test", []string{"rua@example.com"}, "example.com", day, "r.xml.gz", gz)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(msg), "multipart/report") {
		t.Fatalf("%s", msg)
	}

	var sent int
	rep.sender = sendFunc(func(from string, to []string, data []byte) error {
		sent++
		return nil
	})
	if err := rep.SendDue(ctx); err != nil {
		t.Fatal(err)
	}
	if sent != 1 {
		t.Fatalf("sent=%d", sent)
	}
	left, _ := store.ListDMARCAggByDay(ctx, day)
	if len(left) != 0 {
		t.Fatalf("expected deleted, got %d", len(left))
	}
}

type sendFunc func(from string, to []string, data []byte) error

func (f sendFunc) Send(from string, to []string, data []byte) error { return f(from, to, data) }
