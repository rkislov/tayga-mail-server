package storage

import (
	"context"
	"database/sql"
	"errors"
)

type AgentArtifact struct {
	ID, UserID, Scope, Name, MIME string
	ExpiresAt                     int64
	Data                          []byte
}

func (s *Store) SaveAgentArtifacts(ctx context.Context, artifacts []AgentArtifact) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range artifacts {
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO agent_artifacts (id,user_id,scope,name,mime,expires_at,data) VALUES (?,?,?,?,?,?,?)`),
			item.ID, item.UserID, item.Scope, item.Name, item.MIME, item.ExpiresAt, item.Data)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM agent_artifacts WHERE id IN
			(SELECT id FROM agent_artifacts WHERE user_id=? ORDER BY expires_at DESC,id DESC LIMIT 100000 OFFSET 128)`), item.UserID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetAgentArtifact(ctx context.Context, id, user, scope string, now int64) (AgentArtifact, error) {
	var item AgentArtifact
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT name,mime,expires_at,data FROM agent_artifacts WHERE id=? AND user_id=? AND scope=? AND expires_at>?`), id, user, scope, now).
		Scan(&item.Name, &item.MIME, &item.ExpiresAt, &item.Data)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return item, err
}

func (s *Store) CleanupAgentArtifacts(ctx context.Context, now int64) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM agent_artifacts WHERE expires_at<=?`), now)
	return err
}
