package storage

import "context"

// EnsureQuota returns ErrQuotaExceeded if adding sizeBytes would exceed the user's quota.
// quota_bytes <= 0 means unlimited.
func EnsureQuota(ctx context.Context, store Driver, user *User, sizeBytes int64) error {
	if user == nil || user.QuotaBytes <= 0 || sizeBytes <= 0 {
		return nil
	}
	used, err := store.SumMailboxBytes(ctx, user.ID)
	if err != nil {
		return err
	}
	if used+sizeBytes > user.QuotaBytes {
		return ErrQuotaExceeded
	}
	return nil
}
