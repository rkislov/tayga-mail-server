package full

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

// validateChatAttachments checks logical references as well as archive checksums.
// A checksum alone would faithfully preserve a missing or truncated source file.
func validateChatAttachments(ctx context.Context, cfg *config.Config, st *storage.Store) error {
	var exists int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='chat_files'`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil // Snapshots from before the Mattermost attachment migrations.
	}
	var hasDiskColumn int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('chat_files') WHERE name='on_disk'`).Scan(&hasDiskColumn); err != nil {
		return err
	}
	query := `SELECT id,size_bytes,0,LENGTH(content) FROM chat_files`
	if hasDiskColumn != 0 {
		query = `SELECT id,size_bytes,on_disk,LENGTH(content) FROM chat_files`
	}
	rows, err := st.DB().QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	// Use the same explicit/default directory as the HTTP server.
	directory := cfg.MattermostAttachmentsPath()
	for rows.Next() {
		var id string
		var size, blobSize int64
		var onDisk int
		if err := rows.Scan(&id, &size, &onDisk, &blobSize); err != nil {
			return err
		}
		if len(id) != 26 || strings.Trim(id, "abcdefghijklmnopqrstuvwxyz234567") != "" || size <= 0 {
			return errors.New("invalid chat attachment reference")
		}
		if onDisk == 0 {
			if size != blobSize {
				return fmt.Errorf("chat attachment BLOB size mismatch: %s", id)
			}
			continue
		}
		if onDisk != 1 {
			return errors.New("invalid chat attachment storage flag")
		}
		info, err := os.Lstat(filepath.Join(directory, id))
		if err != nil {
			return fmt.Errorf("chat attachment missing: %s: %w", id, err)
		}
		if !info.Mode().IsRegular() || info.Size() != size {
			return fmt.Errorf("chat attachment is not a regular file of the expected size: %s", id)
		}
	}
	return rows.Err()
}
