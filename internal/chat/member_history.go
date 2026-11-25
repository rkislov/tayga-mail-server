package chat

import (
	"context"
	"database/sql"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func (s *Service) recordJoin(ctx context.Context, tx *sql.Tx, channelID, userID string) error {
	_, err := tx.ExecContext(ctx, s.q(`INSERT INTO chat_member_history(id,channel_id,user_id,joined_at) VALUES(?,?,?,?)`), newID(), channelID, userID, time.Now().UnixMilli())
	return err
}
func (s *Service) addMembership(ctx context.Context, tx *sql.Tx, channelID, userID string) error {
	result, err := tx.ExecContext(ctx, s.q(`INSERT INTO chat_members(channel_id,user_id) VALUES(?,?) ON CONFLICT DO NOTHING`), channelID, userID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	return s.recordJoin(ctx, tx, channelID, userID)
}
func (s *Service) RemoveMember(ctx context.Context, actor *storage.User, channelID, userID string) error {
	c, err := s.Channel(ctx, actor, channelID)
	if err != nil {
		return err
	}
	target, err := s.User(ctx, actor.TenantID, userID)
	if err != nil {
		return err
	}
	if c.Kind == "D" || target.ID == c.CreatorID || (actor.ID != target.ID && actor.ID != c.CreatorID) {
		return ErrForbidden
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, s.q(`DELETE FROM chat_members WHERE channel_id=? AND user_id=?`), channelID, target.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, s.q(`UPDATE chat_member_history SET left_at=? WHERE channel_id=? AND user_id=? AND left_at IS NULL`), time.Now().UnixMilli(), channelID, target.ID); err != nil {
		return err
	}
	return tx.Commit()
}
