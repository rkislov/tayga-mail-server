package storage

import (
	"context"
	"time"
)

func (s *Store) UpsertDMARCAgg(ctx context.Context, row *DMARCAggRow) error {
	if row == nil {
		return nil
	}
	now := time.Now().UTC()
	if row.ID == "" {
		row.ID = NewID()
	}
	if row.Count <= 0 {
		row.Count = 1
	}
	if row.Day == "" {
		row.Day = now.Format("2006-01-02")
	}
	row.CreatedAt = now
	row.UpdatedAt = now

	var q string
	if s.dialect == DialectPostgres {
		q = `INSERT INTO dmarc_agg(
			id, domain, day, source_ip, envelope_domain, header_from,
			spf_result, dkim_result, disposition, policy, rua, count, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (domain, day, source_ip, envelope_domain, header_from, spf_result, dkim_result, disposition)
		DO UPDATE SET count = dmarc_agg.count + EXCLUDED.count, updated_at = EXCLUDED.updated_at,
			rua = CASE WHEN EXCLUDED.rua <> '' THEN EXCLUDED.rua ELSE dmarc_agg.rua END,
			policy = CASE WHEN EXCLUDED.policy <> '' THEN EXCLUDED.policy ELSE dmarc_agg.policy END`
	} else {
		q = `INSERT INTO dmarc_agg(
			id, domain, day, source_ip, envelope_domain, header_from,
			spf_result, dkim_result, disposition, policy, rua, count, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT (domain, day, source_ip, envelope_domain, header_from, spf_result, dkim_result, disposition)
		DO UPDATE SET count = count + excluded.count, updated_at = excluded.updated_at,
			rua = CASE WHEN excluded.rua <> '' THEN excluded.rua ELSE rua END,
			policy = CASE WHEN excluded.policy <> '' THEN excluded.policy ELSE policy END`
	}
	_, err := s.db.ExecContext(ctx, q,
		row.ID, row.Domain, row.Day, row.SourceIP, row.EnvelopeDomain, row.HeaderFrom,
		row.SPFResult, row.DKIMResult, row.Disposition, row.Policy, row.RUA, row.Count, now, now,
	)
	return mapErr(err)
}

func (s *Store) ListDMARCAggByDay(ctx context.Context, day string) ([]*DMARCAggRow, error) {
	q := s.rebind(`SELECT id, domain, day, source_ip, envelope_domain, header_from,
		spf_result, dkim_result, disposition, policy, rua, count, created_at, updated_at
		FROM dmarc_agg WHERE day = ? ORDER BY domain, source_ip`)
	rows, err := s.db.QueryContext(ctx, q, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*DMARCAggRow
	for rows.Next() {
		r := &DMARCAggRow{}
		if err := rows.Scan(
			&r.ID, &r.Domain, &r.Day, &r.SourceIP, &r.EnvelopeDomain, &r.HeaderFrom,
			&r.SPFResult, &r.DKIMResult, &r.Disposition, &r.Policy, &r.RUA, &r.Count,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteDMARCAggByDay(ctx context.Context, day string) error {
	q := s.rebind(`DELETE FROM dmarc_agg WHERE day = ?`)
	_, err := s.db.ExecContext(ctx, q, day)
	return err
}

func (s *Store) ListDMARCAggDaysBefore(ctx context.Context, beforeDay string) ([]string, error) {
	q := s.rebind(`SELECT DISTINCT day FROM dmarc_agg WHERE day < ? ORDER BY day`)
	rows, err := s.db.QueryContext(ctx, q, beforeDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var day string
		if err := rows.Scan(&day); err != nil {
			return nil, err
		}
		out = append(out, day)
	}
	return out, rows.Err()
}
