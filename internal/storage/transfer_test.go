package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
)

func TestFullSQLitePostgresTransfer(t *testing.T) {
	cfg := bookingPostgresConfig(t)
	if cfg == nil {
		t.Skip("set TAYGA_TEST_BOOKING_POSTGRES_DSN for isolated PostgreSQL schema")
	}
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "source.sqlite")
	sourceDriver, err := Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: path}})
	if err != nil {
		t.Fatal(err)
	}
	source := sourceDriver.(*Store)
	tenant, err := source.CreateTenant(ctx, "transfer-test")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := source.CreateDomain(ctx, tenant.ID, "transfer.test")
	if err != nil {
		t.Fatal(err)
	}
	user, err := source.CreateUser(ctx, &User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: "alice", Email: "alice@transfer.test", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	box, err := source.EnsureMailbox(ctx, user.ID, "INBOX", "mail/alice")
	if err != nil {
		t.Fatal(err)
	}
	message, err := source.InsertMessage(ctx, &Message{MailboxID: box.ID, FilePath: "unchanged/message.eml", Size: 10, InternalDate: time.Now(), Subject: "transfer private", FromAddr: "fixture@transfer.test", ToAddr: user.Email})
	if err != nil {
		t.Fatal(err)
	}
	if err = source.UpsertMessageSearch(ctx, message.ID, message.Subject, message.FromAddr, message.ToAddr, "unique-search-needle"); err != nil {
		t.Fatal(err)
	}
	// Historical SQLite FTS could retain malformed MIME charset bytes and NUL.
	if _, err = source.DB().ExecContext(ctx, `UPDATE message_fts SET body=? WHERE message_id=?`, "unique-search-needle \xd0\xee\x00 remains", message.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = source.DB().ExecContext(ctx, `INSERT INTO message_fts(message_id,subject,from_addr,to_addr,body) SELECT message_id,subject,from_addr,to_addr,body FROM message_fts WHERE message_id=?`, message.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = source.DB().ExecContext(ctx, `UPDATE messages SET subject=?,from_addr=?,to_addr=?,internal_date=? WHERE id=?`, "old\xff subject", "from\xd0\xee", "to\x00recipient", "2020-02-27 14:04:40 +0500 +0500", message.ID); err != nil {
		t.Fatal(err)
	}
	if err = source.PutSetting(ctx, "mattermost.agents.history:private", `{"messages":["PRIVATE_CONTENT_SENTINEL"]}`); err != nil {
		t.Fatal(err)
	}
	if err = source.PutSetting(ctx, "opaque-state", strings.Repeat("unchanged-state-", 4000)); err != nil {
		t.Fatal(err)
	}
	if _, err = source.CreateOAuthToken(ctx, &OAuthToken{UserID: user.ID, AccessToken: "fixture-access", RefreshToken: "fixture-refresh", ExpiresAt: time.Now().Add(time.Hour), RefreshExpiresAt: time.Now().Add(24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	syncRaw := syncFixtureJSON("preserved-sync-key")
	if err = source.PutFlowSyncState(ctx, user.ID, "ews:transfer", "inbox", syncRaw); err != nil {
		t.Fatal(err)
	}
	targetDriver, err := Open(ctx, *cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer targetDriver.Close()
	target := targetDriver.(*Store)
	if _, err = TransferSQLiteToPostgres(ctx, path, target, nil); err == nil {
		t.Fatal("accepted a running source")
	}
	preflight, err := CheckSQLitePostgresTransfer(ctx, path, target, nil)
	if err != nil || !preflight.Verified || !preflight.CheckOnly {
		t.Fatal("live source preflight", err)
	}
	var untouched int
	if err = target.DB().QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&untouched); err != nil || untouched != 0 {
		t.Fatal("preflight copied application data", err)
	}
	if err = source.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	report, err := TransferSQLiteToPostgres(ctx, path, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Verified || len(report.Tables) < 80 {
		t.Fatalf("incomplete coverage: %d tables", len(report.Tables))
	}
	var duplicates int64
	for _, table := range report.Tables {
		duplicates += table.SkippedDuplicateSearchRows
	}
	if duplicates != 1 {
		t.Fatalf("duplicate search rows: %d", duplicates)
	}
	var corrected int64
	for _, table := range report.Tables {
		corrected += table.NormalizedSearchFields
	}
	if corrected != 1 {
		t.Fatalf("search corrections: %d", corrected)
	}
	var preserved int64
	for _, table := range report.Tables {
		preserved += table.PreservedHeaderFields
	}
	if preserved != 3 {
		t.Fatalf("preserved header fields: %d", preserved)
	}
	var subject, from, to []byte
	if err = target.DB().QueryRowContext(ctx, `SELECT subject,from_addr,to_addr FROM message_header_bytes WHERE message_id=$1`, message.ID).Scan(&subject, &from, &to); err != nil || string(subject) != "old\xff subject" || string(from) != "from\xd0\xee" || string(to) != "to\x00recipient" {
		t.Fatal("original header bytes lost", err)
	}
	var indexed string
	if err = target.DB().QueryRowContext(ctx, `SELECT body FROM message_search WHERE message_id=$1`, message.ID).Scan(&indexed); err != nil || indexed != normalizeSearchText("unique-search-needle \xd0\xee\x00 remains") {
		t.Fatal("derived index encoding", err)
	}
	if err = target.UpsertMessageSearch(ctx, message.ID, message.Subject, message.FromAddr, message.ToAddr, "unique-search-needle \xff\x00 fresh"); err != nil {
		t.Fatal("live search indexing", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("source database changed")
	}
	copied, err := target.GetUserByID(ctx, user.ID)
	if err != nil || copied.Email != user.Email {
		t.Fatal("user/identity lost")
	}
	migratedMessage, err := target.GetMessageByID(ctx, message.ID)
	if err != nil || !migratedMessage.InternalDate.Equal(time.Date(2020, 2, 27, 9, 4, 40, 0, time.UTC)) {
		t.Fatal("legacy numeric timezone lost", err)
	}
	token, err := target.GetOAuthTokenByAccess(ctx, "fixture-access")
	if err != nil || token.UserID != user.ID {
		t.Fatal("session lost")
	}
	setting, ok, err := target.GetSetting(ctx, "opaque-state")
	if err != nil || !ok || setting != strings.Repeat("unchanged-state-", 4000) {
		t.Fatal("opaque data changed")
	}
	hits, err := target.SearchMessages(ctx, user.ID, "", "", "", "", "unique-search-needle", 10)
	if err != nil || len(hits) != 1 {
		t.Fatalf("search not rebuilt: %v (%d)", err, len(hits))
	}
	restoredState, err := target.GetFlowSyncState(ctx, user.ID, "ews:transfer", "inbox")
	if err != nil || restoredState != syncRaw {
		t.Fatal("shared snapshot/replay state lost")
	}
	// Trigger-produced file owners must be preserved exactly and triggers restored.
	var owners int
	if err = target.db.QueryRowContext(ctx, `SELECT count(*) FROM file_owners WHERE user_id=$1`, user.ID).Scan(&owners); err != nil || owners != 1 {
		t.Fatalf("file owner lost/duplicated: %d %v", owners, err)
	}
	second, err := target.CreateUser(ctx, &User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: "bob", Email: "bob@transfer.test", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = target.db.QueryRowContext(ctx, `SELECT count(*) FROM file_owners WHERE user_id=$1`, second.ID).Scan(&owners); err != nil || owners != 1 {
		t.Fatal("file owner trigger not restored")
	}
	if _, err = TransferSQLiteToPostgres(ctx, path, target, nil); err == nil {
		t.Fatal("overwrote a populated destination")
	}
}

func TestTransferNormalizationAndFingerprint(t *testing.T) {
	original := time.Date(2026, 11, 25, 11, 22, 33, 123456789, time.FixedZone("test", 10800))
	normalized, err := normalizeTransferValue(original, "timestamptz")
	if err != nil || !normalized.(time.Time).Equal(original.UTC().Round(time.Microsecond)) {
		t.Fatal("timestamp normalization")
	}
	if _, err = normalizeTransferValue("unsafe\x00text", "text"); err == nil {
		t.Fatal("NUL text silently changed")
	}
	if _, err = normalizeTransferValue("bad\xff", "text"); err == nil {
		t.Fatal("invalid original UTF-8 silently changed")
	}
	var first, second transferDigest
	first.add([]any{"abc", int64(2), nil})
	first.add([]any{"def", int64(3), []byte{0, 1}})
	second.add([]any{"def", int64(3), []byte{0, 1}})
	second.add([]any{"abc", int64(2), nil})
	if first != second {
		t.Fatal("fingerprint depends on SQL iteration order")
	}
	second.add([]any{"abc", int64(2), nil})
	if first == second {
		t.Fatal("fingerprint lost duplicate rows")
	}
}

func TestFullTransferFailureRollsBackEveryTableAndTrigger(t *testing.T) {
	cfg := bookingPostgresConfig(t)
	if cfg == nil {
		t.Skip("set TAYGA_TEST_BOOKING_POSTGRES_DSN")
	}
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "invalid-index.sqlite")
	src, err := Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: path}})
	if err != nil {
		t.Fatal(err)
	}
	// FTS5 cannot enforce the message foreign key. This stale derived row must
	// cause an explicit failure, never a partially committed destination.
	if _, err = src.DB().ExecContext(ctx, `INSERT INTO message_fts(message_id,subject,from_addr,to_addr,body) VALUES('missing-message','fixture','','','orphan')`); err != nil {
		t.Fatal(err)
	}
	if err = src.Close(); err != nil {
		t.Fatal(err)
	}
	dst, err := Open(ctx, *cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()
	target := dst.(*Store)
	report, err := TransferSQLiteToPostgres(ctx, path, target, nil)
	if err == nil || report.Verified {
		t.Fatal("invalid FTS foreign key committed")
	}
	var count int
	if err = target.db.QueryRowContext(ctx, `SELECT count(*) FROM message_search`).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial search rows committed")
	}
	if err = target.db.QueryRowContext(ctx, `SELECT count(*) FROM chat_file_write_lock WHERE id=1`).Scan(&count); err != nil || count != 1 {
		t.Fatal("bootstrap mutex not restored")
	}
	if err = target.db.QueryRowContext(ctx, `SELECT count(*) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE NOT t.tgisinternal AND t.tgenabled='D' AND n.nspname=current_schema()`).Scan(&count); err != nil || count != 0 {
		t.Fatal("trigger changes survived rollback")
	}
}
