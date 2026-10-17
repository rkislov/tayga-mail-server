package storage

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// SetSpamLearningPolicy must be called before accepting client connections.
// The callback may safely read a live immutable settings snapshot.
func (s *Store) SetSpamLearningPolicy(policy func() (bool, string)) { s.learningPolicy = policy }
func (s *Store) InsertCopiedMessage(ctx context.Context, msg *Message, sourceMailboxID string) (*Message, error) {
	return s.insertMessage(ctx, msg, "", "", sourceMailboxID)
}
func spamTransition(from, to, folder string) string {
	isSpam := func(v string) bool {
		return strings.EqualFold(v, "Junk") || strings.EqualFold(v, "Spam") || folder != "" && strings.EqualFold(v, folder)
	}
	if isSpam(from) == isSpam(to) {
		return ""
	}
	if isSpam(to) {
		return "spam"
	}
	if strings.EqualFold(to, "Trash") || strings.EqualFold(to, "Deleted Items") || strings.EqualFold(to, "Quarantine") {
		return ""
	}
	return "ham"
}

// Called in the move/copy transaction: initial SMTP/Sieve delivery never trains.
func (s *Store) queueSpamLearning(ctx context.Context, tx *sql.Tx, id, fromID, toID string) error {
	if s.learningPolicy == nil {
		return nil
	}
	enabled, folder := s.learningPolicy()
	if !enabled {
		return nil
	}
	var from, to, user, other, domain string
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT name,user_id FROM mailboxes WHERE id=?`), fromID).Scan(&from, &other); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT m.name,m.user_id,u.domain_id FROM mailboxes m JOIN users u ON u.id=m.user_id WHERE m.id=?`), toID).Scan(&to, &user, &domain); err != nil {
		return err
	}
	if user != other {
		return nil
	}
	class := spamTransition(from, to, folder)
	if class == "" {
		return nil
	}
	now := time.Now().Unix()
	_, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO spam_learning_jobs(message_id,user_id,domain_id,class,next_attempt,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(message_id) DO UPDATE SET class=excluded.class,revision=spam_learning_jobs.revision+1,attempts=0,next_attempt=excluded.next_attempt,updated_at=excluded.updated_at,state='pending',last_error=''`), id, user, domain, class, now, now)
	return err
}

type SpamLearningJob struct {
	MessageID, UserID, DomainID, Class string
	Revision                           int64
	Attempts                           int
}

func (s *Store) DueSpamLearning(ctx context.Context) ([]SpamLearningJob, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT message_id,user_id,domain_id,class,revision,attempts FROM spam_learning_jobs WHERE state='pending' AND lease_until<=? AND next_attempt<=? ORDER BY next_attempt LIMIT 25`), time.Now().Unix(), time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []SpamLearningJob
	for rows.Next() {
		var j SpamLearningJob
		if err = rows.Scan(&j.MessageID, &j.UserID, &j.DomainID, &j.Class, &j.Revision, &j.Attempts); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
func (s *Store) FinishSpamLearning(ctx context.Context, j SpamLearningJob, outcome, detail string) error {
	state := "done"
	next := int64(0)
	if outcome == "error" {
		state = "pending"
		next = time.Now().Add(time.Duration(1<<min(j.Attempts, 8)) * time.Minute).Unix()
		if j.Attempts >= 7 {
			state = "failed"
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, s.rebind(`UPDATE spam_learning_jobs SET state=?,attempts=attempts+1,next_attempt=?,last_error=?,updated_at=? WHERE message_id=? AND revision=?`), state, next, detail, time.Now().Unix(), j.MessageID, j.Revision)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, s.rebind(`UPDATE spam_learning_jobs SET lease_until=0 WHERE message_id=?`), j.MessageID)
	if err != nil {
		return err
	}
	// Every acknowledged attempt is counted even if a newer correction arrived.
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO protection_counters(domain_id,kind,outcome,bucket,count) VALUES(?,'learning',?,?,1) ON CONFLICT(domain_id,kind,outcome,bucket) DO UPDATE SET count=protection_counters.count+1`), j.DomainID, outcome, time.Now().Unix()/3600*3600)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) RecordProtection(ctx context.Context, domain, kind, outcome string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`INSERT INTO protection_counters(domain_id,kind,outcome,bucket,count) VALUES(?,?,?,?,1) ON CONFLICT(domain_id,kind,outcome,bucket) DO UPDATE SET count=protection_counters.count+1`), domain, kind, outcome, time.Now().Unix()/3600*3600)
	return err
}
func (s *Store) ProtectionStatistics(ctx context.Context, domains []string, since int64) (map[string]any, error) {
	filter := ""
	args := []any{since}
	if domains != nil {
		if len(domains) == 0 {
			filter = " AND 1=0"
		} else {
			filter = " AND domain_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(domains)), ",") + ")"
			for _, d := range domains {
				args = append(args, d)
			}
		}
	}
	counts := map[string]map[string]int64{"learning": {}, "antivirus": {}}
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT kind,outcome,SUM(count) FROM protection_counters WHERE bucket>=?`+filter+` GROUP BY kind,outcome`), args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kind, outcome string
		var n int64
		if err = rows.Scan(&kind, &outcome, &n); err != nil {
			rows.Close()
			return nil, err
		}
		if counts[kind] == nil {
			counts[kind] = map[string]int64{}
		}
		counts[kind][outcome] = n
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = s.db.QueryContext(ctx, s.rebind(`SELECT state,COUNT(*) FROM spam_learning_jobs WHERE 1=1`+filter+` GROUP BY state`), args[1:]...)
	if err != nil {
		return nil, err
	}
	queue := map[string]int64{}
	for rows.Next() {
		var state string
		var n int64
		if err = rows.Scan(&state, &n); err != nil {
			rows.Close()
			return nil, err
		}
		queue[state] = n
	}
	err = rows.Err()
	rows.Close()
	return map[string]any{"counts": counts, "queue": queue}, err
}

// A short lease serializes retries and corrections across multiple processes.
func (s *Store) ClaimSpamLearning(ctx context.Context, j SpamLearningJob) (bool, error) {
	now := time.Now().Unix()
	r, e := s.db.ExecContext(ctx, s.rebind(`UPDATE spam_learning_jobs SET lease_until=? WHERE message_id=? AND revision=? AND state='pending' AND lease_until<=? AND next_attempt<=?`), now+60, j.MessageID, j.Revision, now, now)
	if e != nil {
		return false, e
	}
	n, e := r.RowsAffected()
	return n == 1, e
}
func (s *Store) CleanupProtection(ctx context.Context) {
	cutoff := time.Now().Add(-90 * 24 * time.Hour).Unix()
	_, _ = s.db.ExecContext(ctx, s.rebind(`DELETE FROM protection_counters WHERE bucket<?`), cutoff)
	_, _ = s.db.ExecContext(ctx, s.rebind(`DELETE FROM spam_learning_jobs WHERE state='done' AND updated_at<?`), time.Now().Add(-7*24*time.Hour).Unix())
}
