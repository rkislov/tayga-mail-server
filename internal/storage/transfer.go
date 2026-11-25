package storage

// Full offline transfer copies physical application tables, including tokens,
// opaque synchronization state and extension metadata. Logical mail exports are
// intentionally not used here. Source data is never changed.
import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
	"github.com/tayga/tms/internal/maintenance"
)

type TransferTable struct {
	SkippedDuplicateSearchRows int64  `json:"skipped_duplicate_search_rows,omitempty"`
	PreservedHeaderFields      int64  `json:"preserved_header_fields,omitempty"`
	NormalizedSearchFields     int64  `json:"normalized_search_fields,omitempty"`
	Source                     string `json:"source"`
	Target                     string `json:"target"`
	Rows                       int64  `json:"rows"`
	Digest                     string `json:"sha256_multiset"`
	Verified                   bool   `json:"verified"`
}
type TransferReport struct {
	CheckOnly bool            `json:"check_only,omitempty"`
	Version   int             `json:"version"`
	Tables    []TransferTable `json:"tables"`
	Derived   []string        `json:"derived"`
	Excluded  []string        `json:"excluded"`
	Verified  bool            `json:"verified"`
}
type transferColumn struct{ name, kind string }
type transferPlan struct {
	source, target string
	columns        []transferColumn
}
type transferConstraint struct {
	table, name          string
	deferrable, deferred bool
}
type transferTrigger struct{ table, name, enabled string }

// TransferSQLiteToPostgres requires a stopped SQLite installation and a fresh,
// migrated PostgreSQL database. The destination role must own its tables; it
// does NOT need superuser privileges. All data and temporary constraint/trigger
// changes are committed together, only after complete content verification.
func TransferSQLiteToPostgres(ctx context.Context, path string, target *Store, progress func(TransferTable)) (TransferReport, error) {
	return transferSQLiteToPostgres(ctx, path, target, progress, false)
}

// CheckSQLitePostgresTransfer validates a consistent read-only source snapshot
// while the source server may remain running. Application data is never copied.
func CheckSQLitePostgresTransfer(ctx context.Context, path string, target *Store, progress func(TransferTable)) (TransferReport, error) {
	return transferSQLiteToPostgres(ctx, path, target, progress, true)
}
func transferSQLiteToPostgres(ctx context.Context, path string, target *Store, progress func(TransferTable), checkOnly bool) (report TransferReport, err error) {
	report = TransferReport{CheckOnly: checkOnly, Version: 1, Tables: []TransferTable{}, Derived: []string{"mail_log_fts is rebuilt from mail_log by PostgreSQL queries", "message_search.tsv is regenerated with the PostgreSQL simple dictionary; duplicate SQLite search documents use their latest rowid and skipped duplicates are counted; orphan document references still fail; invalid UTF-8 and NUL are normalized only in derived search text and counted per table", "Malformed subject/from_addr/to_addr display metadata is normalized only with exact original bytes verified in message_header_bytes; counts are reported per table", "SQL timestamps are normalized to UTC at PostgreSQL microsecond precision; raw calendar/JSON/MIME data remains unchanged"}, Excluded: []string{}}
	if target == nil || target.dialect != DialectPostgres {
		return report, errors.New("target must be PostgreSQL")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return report, err
	}
	lock, err := maintenance.SQLite(path, !checkOnly)
	if err != nil {
		return report, err
	}
	defer lock.Close()
	uri := (&url.URL{Scheme: "file", Path: path}).String() + "?mode=ro"
	source, err := sql.Open(sqliteDriverName, uri)
	if err != nil {
		return report, errors.New("cannot open source SQLite")
	}
	defer source.Close()
	source.SetMaxOpenConns(1)
	read, err := source.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return report, err
	}
	defer read.Rollback()
	fk, err := read.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return report, err
	}
	invalid := fk.Next()
	fkErr := fk.Err()
	fk.Close()
	if fkErr != nil {
		return report, errors.New("cannot validate source foreign keys")
	}
	if invalid {
		return report, errors.New("source contains foreign-key violations; no data transferred")
	}
	tx, err := target.db.BeginTx(ctx, nil)
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(1928305711)"); err != nil {
		return report, err
	}
	plans, excluded, err := planTransfer(ctx, read, tx)
	if err != nil {
		return report, err
	}
	report.Excluded = excluded
	constraints, triggers, err := deferTransferConstraints(ctx, tx)
	if err != nil {
		return report, err
	}
	for _, p := range plans {
		var record TransferTable
		var e error
		if checkOnly {
			record, e = validateTransferTable(ctx, read, p)
		} else {
			record, e = copyTransferTable(ctx, read, tx, p, target)
		}
		if e != nil {
			return report, fmt.Errorf("transfer %s: %w", p.source, e)
		}
		report.Tables = append(report.Tables, record)
		if progress != nil {
			progress(record)
		}
	}
	if checkOnly {
		report.Verified = true
		return report, nil
	}
	if _, err = tx.ExecContext(ctx, `UPDATE message_search SET tsv=to_tsvector('simple',coalesce(subject,'') || ' ' || coalesce(from_addr,'') || ' ' || coalesce(to_addr,'') || ' ' || coalesce(body,''))`); err != nil {
		return report, errors.New("cannot rebuild PostgreSQL message search")
	}
	if _, err = tx.ExecContext(ctx, "SET CONSTRAINTS ALL IMMEDIATE"); err != nil {
		return report, errors.New("destination foreign-key validation failed; transaction rolled back")
	}
	for _, c := range constraints {
		state := "NOT DEFERRABLE"
		if c.deferrable {
			state = "DEFERRABLE INITIALLY IMMEDIATE"
			if c.deferred {
				state = "DEFERRABLE INITIALLY DEFERRED"
			}
		}
		if _, err = tx.ExecContext(ctx, "ALTER TABLE "+pq.QuoteIdentifier(c.table)+" ALTER CONSTRAINT "+pq.QuoteIdentifier(c.name)+" "+state); err != nil {
			return report, err
		}
	}
	for _, t := range triggers {
		mode := "ENABLE"
		switch t.enabled {
		case "D":
			mode = "DISABLE"
		case "A":
			mode = "ENABLE ALWAYS"
		case "R":
			mode = "ENABLE REPLICA"
		}
		if _, err = tx.ExecContext(ctx, "ALTER TABLE "+pq.QuoteIdentifier(t.table)+" "+mode+" TRIGGER "+pq.QuoteIdentifier(t.name)); err != nil {
			return report, err
		}
	}
	if err = tx.Commit(); err != nil {
		return report, errors.New("destination commit failed; inspect target before retrying")
	}
	report.Verified = true
	return report, nil
}

func planTransfer(ctx context.Context, source *sql.Tx, target *sql.Tx) ([]transferPlan, []string, error) {
	rows, err := source.QueryContext(ctx, `SELECT name,coalesce(sql,'') FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		return nil, nil, err
	}
	tables := map[string]string{}
	for rows.Next() {
		var name, definition string
		if err = rows.Scan(&name, &definition); err != nil {
			rows.Close()
			return nil, nil, err
		}
		tables[name] = definition
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	columns := map[string][]transferColumn{}
	rows, err = target.QueryContext(ctx, `SELECT table_name,column_name,udt_name FROM information_schema.columns WHERE table_schema=current_schema() ORDER BY table_name,ordinal_position`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var table string
		var c transferColumn
		if err = rows.Scan(&table, &c.name, &c.kind); err != nil {
			rows.Close()
			return nil, nil, err
		}
		columns[table] = append(columns[table], c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	// Fresh schemas contain exactly one bootstrap mutex row. Nothing else may be
	// overwritten, including account/settings rows added by another process.
	for table := range columns {
		if table == "schema_migrations" {
			continue
		}
		var count int64
		if err = target.QueryRowContext(ctx, "SELECT count(*) FROM "+pq.QuoteIdentifier(table)).Scan(&count); err != nil {
			return nil, nil, err
		}
		if table == "chat_file_write_lock" && count == 1 {
			var id int
			if err = target.QueryRowContext(ctx, `SELECT id FROM chat_file_write_lock`).Scan(&id); err != nil || id != 1 {
				return nil, nil, errors.New("unexpected destination mutex row")
			}
			if _, err = target.ExecContext(ctx, `DELETE FROM chat_file_write_lock`); err != nil {
				return nil, nil, err
			}
			continue
		}
		if count != 0 {
			return nil, nil, fmt.Errorf("destination table %s is not empty", table)
		}
	}
	names := []string{}
	for name := range tables {
		names = append(names, name)
	}
	sort.Strings(names)
	plans := []transferPlan{}
	excluded := []string{}
	for _, name := range names {
		if name == "schema_migrations" || strings.HasPrefix(name, "sqlite_") || name == "mail_log_fts" || strings.HasPrefix(name, "mail_log_fts_") || strings.HasPrefix(name, "message_fts_") {
			excluded = append(excluded, name)
			continue
		}
		to := name
		if name == "message_fts" {
			to = "message_search"
		} else if strings.Contains(strings.ToUpper(tables[name]), "CREATE VIRTUAL TABLE") {
			return nil, nil, fmt.Errorf("unsupported virtual table %s", name)
		}
		dest, exists := columns[to]
		if !exists {
			return nil, nil, fmt.Errorf("source table %s has no destination", name)
		}
		destMap := map[string]transferColumn{}
		for _, c := range dest {
			destMap[c.name] = c
		}
		info, e := source.QueryContext(ctx, "PRAGMA table_info("+pq.QuoteIdentifier(name)+")")
		if e != nil {
			return nil, nil, e
		}
		plan := transferPlan{source: name, target: to}
		for info.Next() {
			var cid, notNull, pk int
			var cname, ctype string
			var def any
			if e = info.Scan(&cid, &cname, &ctype, &notNull, &def, &pk); e != nil {
				info.Close()
				return nil, nil, e
			}
			c, ok := destMap[cname]
			if !ok {
				info.Close()
				return nil, nil, fmt.Errorf("source column %s.%s has no destination", name, cname)
			}
			plan.columns = append(plan.columns, c)
		}
		e = info.Err()
		info.Close()
		if e != nil {
			return nil, nil, e
		}
		plans = append(plans, plan)
	}
	return plans, excluded, nil
}
func deferTransferConstraints(ctx context.Context, tx *sql.Tx) ([]transferConstraint, []transferTrigger, error) {
	rows, err := tx.QueryContext(ctx, `SELECT c.relname,k.conname,k.condeferrable,k.condeferred FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE k.contype='f' AND n.nspname=current_schema()`)
	if err != nil {
		return nil, nil, err
	}
	constraints := []transferConstraint{}
	for rows.Next() {
		var c transferConstraint
		if err = rows.Scan(&c.table, &c.name, &c.deferrable, &c.deferred); err != nil {
			rows.Close()
			return nil, nil, err
		}
		constraints = append(constraints, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	triggers := []transferTrigger{}
	rows, err = tx.QueryContext(ctx, `SELECT c.relname,t.tgname,t.tgenabled::text FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE NOT t.tgisinternal AND n.nspname=current_schema()`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var t transferTrigger
		if err = rows.Scan(&t.table, &t.name, &t.enabled); err != nil {
			rows.Close()
			return nil, nil, err
		}
		triggers = append(triggers, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	for _, c := range constraints {
		if _, err = tx.ExecContext(ctx, "ALTER TABLE "+pq.QuoteIdentifier(c.table)+" ALTER CONSTRAINT "+pq.QuoteIdentifier(c.name)+" DEFERRABLE INITIALLY DEFERRED"); err != nil {
			return nil, nil, err
		}
	}
	for _, t := range triggers {
		if _, err = tx.ExecContext(ctx, "ALTER TABLE "+pq.QuoteIdentifier(t.table)+" DISABLE TRIGGER "+pq.QuoteIdentifier(t.name)); err != nil {
			return nil, nil, err
		}
	}
	_, err = tx.ExecContext(ctx, "SET CONSTRAINTS ALL DEFERRED")
	return constraints, triggers, err
}
func copyTransferTable(ctx context.Context, source, target *sql.Tx, plan transferPlan, store *Store) (TransferTable, error) {
	record := TransferTable{Source: plan.source, Target: plan.target}
	names := []string{}
	quoted := []string{}
	for _, c := range plan.columns {
		names = append(names, c.name)
		quoted = append(quoted, pq.QuoteIdentifier(c.name))
	}
	selectColumns := strings.Join(quoted, ",")
	sourceQuery := transferSourceQuery(plan, selectColumns)
	rows, err := source.QueryContext(ctx, sourceQuery)
	if err != nil {
		return record, err
	}
	defer rows.Close()
	statement, err := target.PrepareContext(ctx, pq.CopyIn(plan.target, names...))
	if err != nil {
		return record, err
	}
	defer statement.Close()
	var sourceHash transferDigest
	var headers []legacyMessageHeaders
	var headerBytes int64
	for rows.Next() {
		corrections := transferTextCorrections{}
		if plan.source == "message_fts" {
			corrections.searchFields = &record.NormalizedSearchFields
		}
		if plan.source == "messages" {
			corrections.headerRows = &headers
			corrections.headerFields = &record.PreservedHeaderFields
			corrections.headerBytes = &headerBytes
		}
		values, err := scanTransferRow(rows, plan.columns, corrections)
		if err != nil {
			return record, err
		}
		sourceHash.add(values)
		if _, err = statement.ExecContext(ctx, values...); err != nil {
			return record, fmt.Errorf("COPY failed for row %d (contents suppressed)", record.Rows+1)
		}
		record.Rows++
	}
	if err = rows.Err(); err != nil {
		return record, err
	}
	rows.Close()
	if plan.source == "message_fts" {
		var total int64
		if err = source.QueryRowContext(ctx, `SELECT count(*) FROM message_fts`).Scan(&total); err != nil {
			return record, err
		}
		record.SkippedDuplicateSearchRows = total - record.Rows
	}
	if _, err = statement.ExecContext(ctx); err != nil {
		return record, errors.New("COPY completion failed (contents suppressed)")
	}
	if err = statement.Close(); err != nil {
		return record, err
	}
	for _, h := range headers {
		if err = store.preserveMessageHeadersTx(ctx, target, h); err != nil {
			return record, err
		}
	}
	verified, err := target.QueryContext(ctx, "SELECT "+selectColumns+" FROM "+pq.QuoteIdentifier(plan.target))
	if err != nil {
		return record, err
	}
	defer verified.Close()
	var targetHash transferDigest
	var count int64
	for verified.Next() {
		values, e := scanTransferRow(verified, plan.columns)
		if e != nil {
			return record, e
		}
		targetHash.add(values)
		count++
	}
	if err = verified.Err(); err != nil {
		return record, err
	}
	if count != record.Rows || sourceHash != targetHash {
		return record, errors.New("row count or content fingerprint mismatch")
	}
	record.Digest = hex.EncodeToString(sourceHash[:])
	record.Verified = true
	return record, nil
}

type transferTextCorrections struct {
	searchFields *int64
	headerFields *int64
	headerBytes  *int64
	headerRows   *[]legacyMessageHeaders
}

func scanTransferRow(rows *sql.Rows, columns []transferColumn, options ...transferTextCorrections) ([]any, error) {
	values := make([]any, len(columns))
	pointers := make([]any, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}
	if err := rows.Scan(pointers...); err != nil {
		return nil, err
	}
	var fixes transferTextCorrections
	if len(options) > 0 {
		fixes = options[0]
	}
	h := legacyMessageHeaders{}
	for i, c := range columns {
		if c.name == "id" {
			h.id = fmt.Sprint(values[i])
			if b, ok := values[i].([]byte); ok {
				h.id = string(b)
			}
		}
	}
	for i, c := range columns {
		if fixes.searchFields != nil || fixes.headerRows != nil {
			if c.name == "subject" || c.name == "from_addr" || c.name == "to_addr" || (fixes.searchFields != nil && c.name == "body") {
				var text string
				var ok bool
				switch v := values[i].(type) {
				case string:
					text, ok = v, true
				case []byte:
					text, ok = string(v), true
				}
				if ok {
					clean := normalizeSearchText(text)
					if clean != text {
						if fixes.searchFields != nil {
							(*fixes.searchFields)++
						} else {
							(*fixes.headerFields)++
							*fixes.headerBytes += int64(len(text))
							if *fixes.headerBytes > 32<<20 {
								return nil, errors.New("legacy header archive exceeds safe in-memory transfer limit")
							}
							switch c.name {
							case "subject":
								h.subject = []byte(text)
							case "from_addr":
								h.from = []byte(text)
							case "to_addr":
								h.to = []byte(text)
							}
						}
					}
					values[i] = clean
				}
			}
		}

		value, err := normalizeTransferValue(values[i], c.kind)
		if err != nil {
			return nil, fmt.Errorf("invalid value in column %s (contents suppressed)", c.name)
		}
		values[i] = value
	}
	if h.changed() {
		*fixes.headerRows = append(*fixes.headerRows, h)
	}
	return values, nil
}
func normalizeTransferValue(value any, kind string) (any, error) {
	if value == nil {
		return nil, nil
	}
	if kind == "bytea" {
		if b, ok := value.([]byte); ok {
			return b, nil
		}
		return nil, errors.New("invalid binary value")
	}
	if b, ok := value.([]byte); ok {
		value = string(b)
	}
	if kind == "timestamptz" || kind == "timestamp" || kind == "date" {
		var parsed time.Time
		if err := (sqlTime{&parsed}).Scan(value); err != nil {
			return nil, errors.New("invalid timestamp")
		}
		return parsed.UTC().Round(time.Microsecond), nil
	}
	if kind == "bool" {
		switch v := value.(type) {
		case bool:
			return v, nil
		case int64:
			if v == 0 || v == 1 {
				return v == 1, nil
			}
		}
		return nil, errors.New("invalid boolean")
	}
	if kind == "jsonb" {
		var raw any
		decoder := json.NewDecoder(strings.NewReader(fmt.Sprint(value)))
		decoder.UseNumber()
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		data, err := json.Marshal(raw)
		return string(data), err
	}
	if text, ok := value.(string); ok && (strings.ContainsRune(text, 0) || !utf8.ValidString(text)) {
		return nil, errors.New("PostgreSQL text requires valid UTF-8 without NUL")
	}
	return value, nil
}

// Sum SHA-256 row digests modulo 2^256 so SQL iteration order cannot change a
// fingerprint. Counts and constraints are verified separately (not XOR hashes).
type transferDigest [32]byte

func (d *transferDigest) add(values []any) {
	h := sha256.New()
	for _, value := range values {
		writeTransferValue(h, value)
	}
	sum := h.Sum(nil)
	carry := 0
	for i := 31; i >= 0; i-- {
		n := int(d[i]) + int(sum[i]) + carry
		d[i] = byte(n)
		carry = n >> 8
	}
}
func writeTransferValue(h hash.Hash, value any) {
	if value == nil {
		_, _ = io.WriteString(h, "N;")
		return
	}
	var text string
	switch v := value.(type) {
	case time.Time:
		text = v.UTC().Format(time.RFC3339Nano)
	case []byte:
		_, _ = io.WriteString(h, "B"+strconv.Itoa(len(v))+":")
		_, _ = h.Write(v)
		return
	case string:
		text = v
	default:
		text = fmt.Sprint(v)
	}
	_, _ = io.WriteString(h, "V"+strconv.Itoa(len(text))+":")
	_, _ = io.WriteString(h, text)
}

func transferSourceQuery(plan transferPlan, columns string) string {
	query := "SELECT " + columns + " FROM " + pq.QuoteIdentifier(plan.source)
	if plan.source == "message_fts" {
		query += " WHERE rowid IN (SELECT max(rowid) FROM message_fts GROUP BY message_id)"
	}
	return query
}
func validateTransferTable(ctx context.Context, source *sql.Tx, plan transferPlan) (TransferTable, error) {
	record := TransferTable{Source: plan.source, Target: plan.target}
	if plan.source == "message_fts" {
		var orphan int
		if err := source.QueryRowContext(ctx, `SELECT count(*) FROM message_fts f WHERE NOT EXISTS(SELECT 1 FROM messages m WHERE m.id=f.message_id)`).Scan(&orphan); err != nil {
			return record, err
		}
		if orphan != 0 {
			return record, errors.New("derived search documents reference missing messages")
		}
	}
	quoted := []string{}
	for _, c := range plan.columns {
		quoted = append(quoted, pq.QuoteIdentifier(c.name))
	}
	rows, err := source.QueryContext(ctx, transferSourceQuery(plan, strings.Join(quoted, ",")))
	if err != nil {
		return record, err
	}
	defer rows.Close()
	var hash transferDigest
	var headerBytes int64
	for rows.Next() {
		fixes := transferTextCorrections{}
		var headers []legacyMessageHeaders
		if plan.source == "message_fts" {
			fixes.searchFields = &record.NormalizedSearchFields
		}
		if plan.source == "messages" {
			fixes.headerRows = &headers
			fixes.headerFields = &record.PreservedHeaderFields
			fixes.headerBytes = &headerBytes
		}
		values, e := scanTransferRow(rows, plan.columns, fixes)
		if e != nil {
			return record, e
		}
		hash.add(values)
		record.Rows++
	}
	if err = rows.Err(); err != nil {
		return record, err
	}
	rows.Close()
	if plan.source == "message_fts" {
		var total int64
		if err = source.QueryRowContext(ctx, `SELECT count(*) FROM message_fts`).Scan(&total); err != nil {
			return record, err
		}
		record.SkippedDuplicateSearchRows = total - record.Rows
	}
	record.Digest = hex.EncodeToString(hash[:])
	record.Verified = true
	return record, nil
}
