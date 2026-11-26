package storage

import (
	"context"
	"database/sql"
	"errors"
)

type AgentSchedule struct {
	ID, UserID, Scope, ConversationID, Spec string
	NextAt, LastAt                          int64
	Status                                  string
}

const scheduleColumns = "id,user_id,scope,conversation_id,spec,next_at,last_at,status"

func (s *Store) agentSchedules(ctx context.Context, query string, args ...any) ([]AgentSchedule, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AgentSchedule{}
	for rows.Next() {
		var x AgentSchedule
		if err = rows.Scan(&x.ID, &x.UserID, &x.Scope, &x.ConversationID, &x.Spec, &x.NextAt, &x.LastAt, &x.Status); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) ListAgentSchedules(ctx context.Context, user, scope string) ([]AgentSchedule, error) {
	return s.agentSchedules(ctx, "SELECT "+scheduleColumns+" FROM agent_schedules WHERE user_id=? AND scope=? ORDER BY id", user, scope)
}

func (s *Store) DueAgentSchedules(ctx context.Context, now int64) ([]AgentSchedule, error) {
	return s.agentSchedules(ctx, "SELECT "+scheduleColumns+" FROM agent_schedules WHERE next_at>0 AND next_at<=? AND lease_until<=? ORDER BY next_at LIMIT 16", now, now)
}

func (s *Store) CreateAgentSchedule(ctx context.Context, x AgentSchedule) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`INSERT INTO agent_schedules (id,user_id,scope,conversation_id,spec,next_at)
		SELECT ?,?,?,?,?,? WHERE (SELECT COUNT(*) FROM agent_schedules WHERE user_id=?) < 20`),
		x.ID, x.UserID, x.Scope, x.ConversationID, x.Spec, x.NextAt, x.UserID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrAlreadyExists
	}
	return nil
}

func (s *Store) UpdateAgentSchedule(ctx context.Context, x AgentSchedule) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`UPDATE agent_schedules SET spec=?, next_at=?, lease_until=0, claim=''
		WHERE id=? AND user_id=? AND scope=?`), x.Spec, x.NextAt, x.ID, x.UserID, x.Scope)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteAgentSchedule(ctx context.Context, id, user, scope string) error {
	result, err := s.db.ExecContext(ctx, s.rebind("DELETE FROM agent_schedules WHERE id=? AND user_id=? AND scope=?"), id, user, scope)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ClaimAgentSchedule(ctx context.Context, x AgentSchedule, claim string, now, until int64) (bool, error) {
	result, err := s.db.ExecContext(ctx, s.rebind(`UPDATE agent_schedules SET claim=?, lease_until=?
		WHERE id=? AND next_at=? AND spec=? AND lease_until<=?`), claim, until, x.ID, x.NextAt, x.Spec, now)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (s *Store) AgentScheduleClaimActive(ctx context.Context, id, claim string, now int64) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, s.rebind("SELECT 1 FROM agent_schedules WHERE id=? AND claim=? AND lease_until>?"), id, claim, now).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) FinishAgentSchedule(ctx context.Context, id, claim, status string, next, last, retryAt int64) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE agent_schedules SET claim='', lease_until=?, next_at=?, last_at=?, status=?
		WHERE id=? AND claim=?`), retryAt, next, last, status, id, claim)
	return err
}

func (s *Store) DeleteAgentConversationSchedules(ctx context.Context, user, scope, conversation string) error {
	_, err := s.db.ExecContext(ctx, s.rebind("DELETE FROM agent_schedules WHERE user_id=? AND scope=? AND conversation_id=?"), user, scope, conversation)
	return err
}
