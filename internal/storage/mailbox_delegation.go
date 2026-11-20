package storage

import (
	"context"
	"database/sql"
	"strings"
)

// Recheck an EWS mutation's exact source/destination rights in its transaction.
func (s *Store) mailboxRightTx(ctx context.Context, tx *sql.Tx, actorID, mailboxID, right string) error {
	var ownerID, name string
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT user_id,name FROM mailboxes WHERE id=?`), mailboxID).Scan(&ownerID, &name); err != nil {
		return mapErr(err)
	}
	if ownerID == actorID {
		return nil
	}
	var read, manage, enabled, ownerEnabled any
	var inbox, actorTenant, ownerTenant string
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT d.can_read,d.can_manage_mail,d.ews_inbox_permission,actor.enabled,owner.enabled,actor.tenant_id,owner.tenant_id FROM users actor JOIN users owner ON owner.id=? LEFT JOIN mailbox_delegates d ON d.delegate_id=actor.id AND d.owner_id=owner.id WHERE actor.id=?`), ownerID, actorID).Scan(&read, &manage, &inbox, &enabled, &ownerEnabled, &actorTenant, &ownerTenant)
	if err == nil && asBool(enabled) && asBool(ownerEnabled) && actorTenant == ownerTenant {
		if asBool(manage) || name == "INBOX" && inbox == "Editor" {
			return nil
		}
		if (asBool(read) || name == "INBOX" && inbox == "Reviewer") && strings.Contains("lrs", right) {
			return nil
		}
	}
	var rights string
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT rights FROM mailbox_acl WHERE mailbox_id=? AND grantee_user_id=?`), mailboxID, actorID).Scan(&rights); err != nil {
		return ErrUnauthorized
	}
	if !strings.Contains(rights, right) {
		return ErrUnauthorized
	}
	return nil
}
