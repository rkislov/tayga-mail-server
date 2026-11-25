package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (s *Store) putCompactSyncState(ctx context.Context, user, client, collection, raw string, updated time.Time, preserveActivity bool) (bool, error) {
	encoded, err := encodeSyncState(user, client, collection, raw)
	if err != nil {
		return false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if !preserveActivity {
		var retired int
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM flowsync_devices WHERE id=? AND user_id=? AND retired_at IS NOT NULL`), client, user).Scan(&retired); err != nil {
			return false, err
		}
		if retired > 0 {
			return false, ErrUnauthorized
		}
	}
	var snapshot, response any
	for _, blob := range []syncStateBlob{{id: encoded.id, data: encoded.data, size: int64(encoded.rawBytes)}, {id: encoded.responseID, data: encoded.responseData, size: int64(encoded.responseBytes)}} {
		if blob.id == "" {
			continue
		}
		if _, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO flowsync_state_blobs(id,user_id,collection_id,data,raw_bytes,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`), blob.id, user, collection, blob.data, blob.size, time.Now().UTC()); err != nil {
			return false, err
		}
	}
	if encoded.id != "" {
		snapshot = encoded.id
	}
	if encoded.responseID != "" {
		response = encoded.responseID
	}

	if preserveActivity {
		// Compare the complete old value: a concurrent Sync update must win. Do not
		// refresh age merely because an operator compacted the representation.
		result, e := tx.ExecContext(ctx, s.rebind(`UPDATE flowsync_states SET state=?,snapshot_id=?,response_id=? WHERE user_id=? AND client_id=? AND collection_id=? AND state=? AND snapshot_id IS NULL AND response_id IS NULL`), encoded.state, snapshot, response, user, client, collection, raw)
		if e != nil {
			return false, e
		}
		n, e := result.RowsAffected()
		if e != nil {
			return false, e
		}
		if n == 0 {
			return false, nil
		}
		var saved storedSyncState
		if e = tx.QueryRowContext(ctx, s.rebind("SELECT "+syncStateColumns+syncStateFrom+" WHERE s.user_id=? AND s.client_id=? AND s.collection_id=?"), user, client, collection).Scan(saved.targets()...); e != nil {
			return false, e
		}
		decoded, e := saved.decode(user, collection)
		if e != nil || decoded != raw {
			return false, errors.New("synchronization compaction verification failed")
		}

	} else {
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO flowsync_states(user_id,client_id,collection_id,state,snapshot_id,response_id,updated_at,seen_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(user_id,client_id,collection_id) DO UPDATE SET state=excluded.state,snapshot_id=excluded.snapshot_id,response_id=excluded.response_id,updated_at=excluded.updated_at,seen_at=excluded.seen_at`), user, client, collection, encoded.state, snapshot, response, updated, updated)
		if err != nil {
			return false, err
		}
	}
	return encoded.state != raw, tx.Commit()
}

type SyncCompactionReport struct {
	Examined      int64 `json:"examined"`
	Compacted     int64 `json:"compacted"`
	LogicalBytes  int64 `json:"logical_bytes"`
	TemplateBytes int64 `json:"template_bytes"`
	Snapshots     int64 `json:"snapshots"`
	SnapshotBytes int64 `json:"snapshot_bytes"`
	Verified      bool  `json:"verified"`
}

// CompactSyncStates changes representation only. It does not expire a client,
// change updated_at, remove replay responses or alter submission/task receipts.
func (s *Store) CompactSyncStates(ctx context.Context, progress func(int64)) (report SyncCompactionReport, err error) {
	lastUser, lastClient, lastCollection := "", "", ""
	for {
		// Keyset pagination keeps memory bounded and avoids a live cursor while the
		// SQLite driver (one connection) writes the compacted representation.
		rows, e := s.db.QueryContext(ctx, s.rebind(`SELECT user_id,client_id,collection_id,state,updated_at FROM flowsync_states WHERE (user_id,client_id,collection_id)>(?,?,?) ORDER BY user_id,client_id,collection_id LIMIT 32`), lastUser, lastClient, lastCollection)
		if e != nil {
			return report, e
		}
		type item struct {
			u, c, id, raw string
			updated       time.Time
		}
		batch := []item{}
		for rows.Next() {
			var v item
			if e = rows.Scan(&v.u, &v.c, &v.id, &v.raw, &v.updated); e != nil {
				rows.Close()
				return report, e
			}
			batch = append(batch, v)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return report, e
		}
		if len(batch) == 0 {
			break
		}
		for _, v := range batch {
			report.Examined++
			lastUser, lastClient, lastCollection = v.u, v.c, v.id
			if !strings.HasPrefix(v.raw, syncStateEnvelopePrefix) {
				changed, e := s.putCompactSyncState(ctx, v.u, v.c, v.id, v.raw, v.updated, true)
				if e != nil {
					return report, e
				}
				if changed {
					report.Compacted++
				}
			}
			// The ordinary read path reconstructs the exact old JSON and checks both
			// the snapshot digest and scope before compaction is accepted.
			decoded, e := s.readCompactSyncState(ctx, v.u, v.c, v.id, false)
			if e != nil {
				return report, e
			}

			report.LogicalBytes += int64(len(decoded))
			if progress != nil && report.Examined%100 == 0 {
				progress(report.Examined)
			}
		}
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(LENGTH(state)),0) FROM flowsync_states`).Scan(&report.TemplateBytes); err != nil {
		return report, err
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(LENGTH(data)),0) FROM flowsync_state_blobs`).Scan(&report.Snapshots, &report.SnapshotBytes); err != nil {
		return report, err
	}
	report.Verified = true
	return report, nil
}
func (s *Store) readCompactSyncState(ctx context.Context, user, client, collection string, touch bool) (string, error) {
	var saved storedSyncState
	err := s.db.QueryRowContext(ctx, s.rebind("SELECT "+syncStateColumns+syncStateFrom+" WHERE s.user_id=? AND s.client_id=? AND s.collection_id=?"), user, client, collection).Scan(saved.targets()...)

	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	decoded, err := saved.decode(user, collection)
	if err != nil {
		return "", err
	}
	if touch && strings.HasPrefix(client, "ews:") {
		now := time.Now().UTC()
		_, _ = s.db.ExecContext(ctx, s.rebind(`UPDATE flowsync_states SET seen_at=? WHERE user_id=? AND client_id=? AND collection_id=? AND (seen_at IS NULL OR seen_at<?)`), now, user, client, collection, now.Add(-time.Hour))
	}
	return decoded, nil
}

// Only EWS synchronization contexts expire. Submission and task idempotency
// receipts, EAS devices, mail and calendars are never removed by this policy.
func (s *Store) CleanupSyncStates(ctx context.Context, idle time.Duration) (expired, orphaned int64, err error) {
	if idle < 30*24*time.Hour {
		return 0, 0, errors.New("sync state retention must be at least 30 days")
	}
	cutoff := time.Now().UTC().Add(-idle)
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM flowsync_states WHERE client_id LIKE 'ews:%' AND COALESCE(seen_at,updated_at)<? AND (user_id,client_id,collection_id) IN (SELECT user_id,client_id,collection_id FROM flowsync_states WHERE client_id LIKE 'ews:%' AND COALESCE(seen_at,updated_at)<? ORDER BY COALESCE(seen_at,updated_at) LIMIT 1000)`), cutoff, cutoff)
	if err != nil {
		return 0, 0, err
	}
	expired, err = result.RowsAffected()
	if err != nil {
		return expired, 0, err
	}
	result, err = s.db.ExecContext(ctx, s.rebind(`DELETE FROM flowsync_state_blobs WHERE created_at<? AND NOT EXISTS(SELECT 1 FROM flowsync_states s WHERE s.snapshot_id=flowsync_state_blobs.id OR s.response_id=flowsync_state_blobs.id)`), time.Now().UTC().Add(-7*24*time.Hour))
	if err != nil {
		return expired, 0, err
	}
	orphaned, err = result.RowsAffected()
	return
}
