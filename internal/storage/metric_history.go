package storage

import (
	"context"
	"encoding/json"
)

// MetricSample contains numeric counters only, without user or message content.
type MetricSample struct {
	At     int64              `json:"at"`
	Boot   string             `json:"boot"`
	Values map[string]float64 `json:"values"`
}

func (s *Store) SaveMetricSample(ctx context.Context, scope string, sample MetricSample) error {
	data, err := json.Marshal(sample)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO metric_history(scope,recorded_at,payload) VALUES(?,?,?) ON CONFLICT(scope,recorded_at) DO UPDATE SET payload=excluded.payload`), scope, sample.At, string(data))
	return err
}
func (s *Store) MetricHistory(ctx context.Context, scope string, since int64) ([]MetricSample, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT payload FROM metric_history WHERE scope=? AND recorded_at>=? ORDER BY recorded_at DESC LIMIT 1502`), scope, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MetricSample{}
	for rows.Next() {
		var raw string
		var sample MetricSample
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &sample); err != nil {
			return nil, err
		}
		out = append(out, sample)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}
func (s *Store) PruneMetricHistory(ctx context.Context, before int64) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM metric_history WHERE recorded_at<?`), before)
	return err
}
