// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"
	"database/sql"
	"errors"
)

// InboxRuleSet stores EWS metadata and executable Sieve as one atomic revision.
// It runs before the user's active Sieve script; editing either never replaces the other.
type InboxRuleSet struct {
	Revision  int64  `json:"revision"`
	RulesJSON string `json:"rules_json"`
	Script    string `json:"script"`
}

var ErrRulesConflict = errors.New("inbox rules changed; reload and retry")

func (s *Store) GetInboxRules(ctx context.Context, userID string) (InboxRuleSet, error) {
	var v InboxRuleSet
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT revision,rules_json,script FROM inbox_rule_sets WHERE user_id=?`), userID).Scan(&v.Revision, &v.RulesJSON, &v.Script)
	if errors.Is(err, sql.ErrNoRows) {
		return InboxRuleSet{RulesJSON: "[]"}, nil
	}
	return v, err
}
func (s *Store) SaveInboxRules(ctx context.Context, userID string, previous int64, rulesJSON, script string) error {
	var result sql.Result
	var err error
	if previous == 0 {
		result, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO inbox_rule_sets(user_id,revision,rules_json,script) VALUES(?,1,?,?) ON CONFLICT(user_id) DO NOTHING`), userID, rulesJSON, script)
	} else {
		result, err = s.db.ExecContext(ctx, s.rebind(`UPDATE inbox_rule_sets SET revision=revision+1,rules_json=?,script=? WHERE user_id=? AND revision=?`), rulesJSON, script, userID, previous)
	}
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrRulesConflict
	}
	return nil
}
