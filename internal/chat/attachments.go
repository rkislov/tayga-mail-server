package chat

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/tayga/tms/internal/storage"
)

const (
	MaxAttachmentBytes   = 1 << 30
	MaxFilesPerPost      = 5
	maxAttachmentStorage = 4 << 30
	pendingAttachmentTTL = 24 * time.Hour
	imageHeaderLimit     = 1 << 20
)

var (
	ErrAttachmentStorageFull = errors.New("chat attachment storage is full")
	ErrAttachmentTooLarge    = errors.New("attachment upload exceeds 1 GiB")
	attachmentID             = regexp.MustCompile(`^[a-z2-7]{26}$`)
)

type Attachment struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	PostID    string `json:"post_id"`
	CreatedAt int64  `json:"create_at"`
	Name      string `json:"name"`
	MIMEType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// StagedUpload owns a temporary file until it is committed or discarded.
type StagedUpload struct {
	file      Attachment
	temporary string
}

func (u StagedUpload) Size() int64    { return u.file.Size }
func (u StagedUpload) Discard()       { _ = os.Remove(u.temporary) }
func (a Attachment) HasPreview() bool { return a.Width > 0 && a.Height > 0 }

func (s *Service) SetAttachmentDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	s.attachmentDir = dir
	return nil
}

func (s *Service) AttachmentsEnabled() bool { return s.attachmentDir != "" }

func (s *Service) CheckAttachmentSpace() error {
	// Leave 2 GiB free after at most two simultaneous 1 GiB uploads.
	available, err := attachmentFreeBytes(s.attachmentDir)
	if err != nil {
		return err
	}
	if available < 4*MaxAttachmentBytes {
		return ErrAttachmentStorageFull
	}
	return nil
}

func (s *Service) StageAttachment(name string, content io.Reader, limit int64) (StagedUpload, error) {
	name = strings.TrimSpace(path.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == ".." || name == "/" || len(name) > 240 ||
		strings.ContainsFunc(name, unicode.IsControl) {
		return StagedUpload{}, ErrInvalid
	}
	if limit <= 0 || limit > MaxAttachmentBytes {
		return StagedUpload{}, ErrAttachmentTooLarge
	}
	f, err := os.CreateTemp(s.attachmentDir, "upload-")
	if err != nil {
		return StagedUpload{}, err
	}
	staged := StagedUpload{temporary: f.Name()}
	keep := false
	defer func() {
		_ = f.Close()
		if !keep {
			staged.Discard()
		}
	}()
	size, err := io.Copy(f, io.LimitReader(content, limit+1))
	if err != nil {
		return StagedUpload{}, err
	}
	if size > limit {
		return StagedUpload{}, ErrAttachmentTooLarge
	}
	if size == 0 {
		return StagedUpload{}, ErrInvalid
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return StagedUpload{}, err
	}
	var prefix [512]byte
	count, err := io.ReadFull(f, prefix[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return StagedUpload{}, err
	}
	a := Attachment{ID: newID(), Name: name, MIMEType: http.DetectContentType(prefix[:count]), Size: size}
	switch a.MIMEType {
	case "image/png", "image/jpeg", "image/gif":
		a.Width, a.Height = attachmentImageDimensions(f)
	}
	if err := f.Sync(); err != nil {
		return StagedUpload{}, err
	}
	if err := f.Close(); err != nil {
		return StagedUpload{}, err
	}
	staged.file = a
	keep = true
	return staged, nil
}

func attachmentImageDimensions(r io.ReaderAt) (int, int) {
	// Read only the header. This accommodates common 24 MP and 48 MP photos.
	config, format, err := image.DecodeConfig(io.NewSectionReader(r, 0, imageHeaderLimit))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 8192 ||
		config.Height > 8192 || config.Width*config.Height > 50_000_000 {
		return 0, 0
	}
	if format == "jpeg" && jpegSwapsAxes(io.NewSectionReader(r, 0, imageHeaderLimit)) {
		return config.Height, config.Width
	}
	return config.Width, config.Height
}

// RepairAttachmentImageDimensions restores preview metadata for earlier uploads.
func (s *Service) RepairAttachmentImageDimensions(ctx context.Context) error {
	const needsDimensions = `(mime_type='image/jpeg' OR ((width=0 OR height=0) AND mime_type IN ('image/png','image/gif')))`
	type candidate struct {
		id     string
		onDisk bool
	}
	for cursor := ""; ; {
		rows, err := s.db.QueryContext(ctx, s.q(`SELECT id,on_disk FROM chat_files WHERE `+needsDimensions+
			` AND id>? ORDER BY id LIMIT 100`), cursor)
		if err != nil {
			return err
		}
		var batch []candidate
		for rows.Next() {
			var file candidate
			if err := rows.Scan(&file.id, &file.onDisk); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, file)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return s.repairPostAttachmentDimensions(ctx)
		}
		for _, file := range batch {
			cursor = file.id
			var width, height int
			if file.onDisk {
				if !s.AttachmentsEnabled() || !attachmentID.MatchString(file.id) {
					continue
				}
				content, err := os.Open(filepath.Join(s.attachmentDir, file.id))
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				if err != nil {
					return err
				}
				width, height = attachmentImageDimensions(content)
				content.Close()
			} else {
				var prefix []byte
				err := s.db.QueryRowContext(ctx, s.q(`SELECT SUBSTR(content,1,?) FROM chat_files WHERE id=? AND on_disk=0`),
					imageHeaderLimit, file.id).Scan(&prefix)
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				if err != nil {
					return err
				}
				width, height = attachmentImageDimensions(bytes.NewReader(prefix))
			}
			if width == 0 {
				continue
			}
			if _, err := s.db.ExecContext(ctx, s.q(`UPDATE chat_files SET width=?,height=? WHERE id=? AND (width<>? OR height<>?)`),
				width, height, file.id, width, height); err != nil {
				return err
			}
		}
	}
}

// Post history carries a snapshot of each attachment, including its dimensions.
func (s *Service) repairPostAttachmentDimensions(ctx context.Context) error {
	type postSnapshot struct{ id, attachments string }
	for cursor := ""; ; {
		rows, err := s.db.QueryContext(ctx, s.q(`SELECT p.id,p.attachments FROM chat_posts p WHERE p.id>?
			AND EXISTS (SELECT 1 FROM chat_files f WHERE f.post_id=p.id AND f.width>0 AND f.height>0)
			ORDER BY p.id LIMIT 100`), cursor)
		if err != nil {
			return err
		}
		var batch []postSnapshot
		for rows.Next() {
			var post postSnapshot
			if err := rows.Scan(&post.id, &post.attachments); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, post)
		}
		err = rows.Err()
		rows.Close()
		if err != nil || len(batch) == 0 {
			return err
		}
		for _, post := range batch {
			cursor = post.id
			if err := s.repairPostAttachmentSnapshot(ctx, post.id, post.attachments); err != nil {
				return err
			}
		}
	}
}

func (s *Service) repairPostAttachmentSnapshot(ctx context.Context, postID, snapshot string) error {
	var files []Attachment
	if err := json.Unmarshal([]byte(snapshot), &files); err != nil {
		return err
	}
	changed := false
	for i := range files {
		file := &files[i]
		var width, height int
		err := s.db.QueryRowContext(ctx, s.q(`SELECT width,height FROM chat_files
			WHERE id=? AND post_id=? AND width>0 AND height>0`), file.ID, postID).Scan(&width, &height)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if file.Width == width && file.Height == height {
			continue
		}
		file.Width, file.Height = width, height
		changed = true
	}
	if !changed {
		return nil
	}
	repaired, err := json.Marshal(files)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, s.q("UPDATE chat_posts SET attachments=? WHERE id=? AND attachments=?"),
		string(repaired), postID, snapshot)
	return err
}

func (s *Service) UploadAttachments(ctx context.Context, u *storage.User, channel string, uploads []StagedUpload) ([]Attachment, error) {
	if _, err := s.Channel(ctx, u, channel); err != nil {
		return nil, err
	}
	if len(uploads) == 0 || len(uploads) > MaxFilesPerPost {
		return nil, ErrInvalid
	}
	now := time.Now().UnixMilli()
	files := make([]Attachment, 0, len(uploads))
	var incoming int64
	for _, upload := range uploads {
		file := upload.file
		file.ChannelID, file.UserID, file.CreatedAt = channel, u.ID, now
		incoming += file.Size
		files = append(files, file)
	}
	if incoming > MaxAttachmentBytes {
		return nil, ErrAttachmentTooLarge
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "UPDATE chat_file_write_lock SET id=id WHERE id=1"); err != nil {
		return nil, err
	}
	var used int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(size_bytes),0) FROM chat_files").Scan(&used); err != nil {
		return nil, err
	}
	if used+incoming > maxAttachmentStorage {
		return nil, ErrAttachmentStorageFull
	}
	// Publish files before metadata commit. A crash between the two steps leaves
	// an unreferenced file for the sweeper rather than a broken attachment.
	published := make([]string, 0, len(files))
	commitStarted := false
	defer func() {
		if !commitStarted {
			for _, file := range published {
				_ = os.Remove(file)
			}
		}
	}()
	for i, file := range files {
		_, err := tx.ExecContext(ctx, s.q(`INSERT INTO chat_files(id,channel_id,user_id,created_at,name,mime_type,size_bytes,width,height,content,on_disk)
            VALUES(?,?,?,?,?,?,?,?,?,?,1)`), file.ID, file.ChannelID, file.UserID, file.CreatedAt, file.Name,
			file.MIMEType, file.Size, file.Width, file.Height, []byte{})
		if err != nil {
			return nil, err
		}
		finalPath := filepath.Join(s.attachmentDir, file.ID)
		if err := os.Rename(uploads[i].temporary, finalPath); err != nil {
			return nil, err
		}
		published = append(published, finalPath)
	}
	if err := syncAttachmentDir(s.attachmentDir); err != nil {
		return nil, err
	}
	// Leave files in place if the database cannot confirm the commit outcome.
	commitStarted = true
	return files, tx.Commit()
}

func sameAttachmentIDs(files []Attachment, ids []string) bool {
	if len(files) != len(ids) {
		return false
	}
	for i, file := range files {
		if file.ID != ids[i] {
			return false
		}
	}
	return true
}

const attachmentColumns = "f.id,f.channel_id,f.user_id,COALESCE(f.post_id,''),f.created_at,f.name,f.mime_type,f.size_bytes,f.width,f.height"

func (a *Attachment) scanFields() []any {
	return []any{&a.ID, &a.ChannelID, &a.UserID, &a.PostID, &a.CreatedAt, &a.Name, &a.MIMEType, &a.Size, &a.Width, &a.Height}
}

func (s *Service) attachFiles(ctx context.Context, tx *sql.Tx, u *storage.User, post Post, ids []string) ([]Attachment, error) {
	files := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		result, err := tx.ExecContext(ctx, s.q(`UPDATE chat_files SET post_id=? WHERE id=? AND user_id=?
            AND channel_id=? AND post_id IS NULL AND created_at>=?`), post.ID, id, u.ID, post.ChannelID,
			post.CreatedAt-pendingAttachmentTTL.Milliseconds())
		if err != nil {
			return nil, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if count != 1 {
			return nil, ErrInvalid
		}
		var f Attachment
		if err := tx.QueryRowContext(ctx, s.q("SELECT "+attachmentColumns+" FROM chat_files f WHERE f.id=?"), id).Scan(f.scanFields()...); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

type AttachmentContent interface {
	io.ReadSeeker
	io.Closer
}

type legacyContent struct{ *bytes.Reader }

func (legacyContent) Close() error { return nil }

func (s *Service) Attachment(ctx context.Context, u *storage.User, id string, includeContent bool) (Attachment, AttachmentContent, error) {
	var f Attachment
	var onDisk bool
	var data []byte
	fields := append(f.scanFields(), &onDisk)
	columns := attachmentColumns + ",f.on_disk"
	if includeContent {
		columns += ",CASE WHEN f.on_disk=0 THEN f.content ELSE NULL END"
		fields = append(fields, &data)
	}
	err := s.db.QueryRowContext(ctx, s.q(`SELECT `+columns+` FROM chat_files f
        JOIN chat_channels c ON c.id=f.channel_id
        JOIN chat_members m ON m.channel_id=f.channel_id AND m.user_id=?
        WHERE f.id=? AND c.tenant_id=? AND (f.post_id IS NOT NULL OR (f.user_id=? AND f.created_at>=?))`),
		u.ID, id, u.TenantID, u.ID, time.Now().Add(-pendingAttachmentTTL).UnixMilli()).Scan(fields...)
	if errors.Is(err, sql.ErrNoRows) {
		err = storage.ErrNotFound
	}
	if err != nil || !includeContent {
		return f, nil, err
	}
	if !onDisk {
		return f, legacyContent{bytes.NewReader(data)}, nil
	}
	if !s.AttachmentsEnabled() || !attachmentID.MatchString(f.ID) {
		return f, nil, storage.ErrNotFound
	}
	content, err := os.Open(filepath.Join(s.attachmentDir, f.ID))
	if errors.Is(err, os.ErrNotExist) {
		err = storage.ErrNotFound
	}
	return f, content, err
}

// CleanupAttachments removes expired drafts and old files with no metadata,
// including uploads interrupted by a process restart or deleted via a cascade.
func (s *Service) CleanupAttachments(ctx context.Context, now int64) error {
	if _, err := s.db.ExecContext(ctx, s.q("DELETE FROM chat_files WHERE post_id IS NULL AND created_at<?"), now-pendingAttachmentTTL.Milliseconds()); err != nil {
		return err
	}
	if !s.AttachmentsEnabled() {
		return nil
	}
	entries, err := os.ReadDir(s.attachmentDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		id := entry.Name()
		staged := strings.HasPrefix(id, "upload-")
		if !entry.Type().IsRegular() || (!staged && !attachmentID.MatchString(id)) {
			continue
		}
		info, err := entry.Info()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		// Allow ample time for in-flight transfers and commits.
		if now-info.ModTime().UnixMilli() < (2 * pendingAttachmentTTL).Milliseconds() {
			continue
		}
		if !staged {
			var exists int
			err = s.db.QueryRowContext(ctx, s.q("SELECT 1 FROM chat_files WHERE id=? AND on_disk=1"), id).Scan(&exists)
			if err == nil {
				continue
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		if err := os.Remove(filepath.Join(s.attachmentDir, id)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
