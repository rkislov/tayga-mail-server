package mattermost

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func fileModel(file chat.Attachment) map[string]any {
	return map[string]any{
		"id": file.ID, "post_id": file.PostID, "channel_id": file.ChannelID, "user_id": chat.ID(file.UserID),
		"create_at": file.CreatedAt, "update_at": file.CreatedAt, "delete_at": 0,
		"name": file.Name, "extension": strings.TrimPrefix(strings.ToLower(path.Ext(file.Name)), "."),
		"mime_type": file.MIMEType, "size": file.Size, "width": file.Width, "height": file.Height,
		"has_preview_image": file.HasPreview(),
	}
}

func fileModels(files []chat.Attachment) []map[string]any {
	out := make([]map[string]any, 0, len(files))
	for _, file := range files {
		out = append(out, fileModel(file))
	}
	return out
}

func (s *API) files(w http.ResponseWriter, r *http.Request, u *storage.User, route string) {
	if route == "/files" {
		if r.Method != http.MethodPost {
			wrongMethod(w)
			return
		}
		s.uploadFiles(w, r, u)
		return
	}
	parts := strings.Split(strings.TrimPrefix(route, "/files/"), "/")
	if len(parts) > 2 || parts[0] == "" {
		s.unsupported(w, r)
		return
	}
	kind := ""
	if len(parts) == 2 {
		kind = parts[1]
		if kind != "info" && kind != "preview" && kind != "thumbnail" {
			s.unsupported(w, r)
			return
		}
	}
	if r.Method != http.MethodGet && (r.Method != http.MethodHead || kind == "info") {
		wrongMethod(w)
		return
	}
	// Limit simultaneous downloads, including reads of legacy database blobs.
	if kind != "info" && r.Method != http.MethodHead && !s.acquireFileRequest(w) {
		return
	}
	if kind != "info" && r.Method != http.MethodHead {
		defer func() { <-s.fileRequests }()
	}
	file, content, err := s.chat.Attachment(r.Context(), u, parts[0], kind != "info" && r.Method != http.MethodHead)
	if err != nil {
		s.failure(w, err)
		return
	}
	if content != nil {
		defer content.Close()
	}
	if kind == "info" {
		write(w, http.StatusOK, fileModel(file))
		return
	}
	contentType, disposition := "application/octet-stream", "attachment"
	if kind == "preview" || kind == "thumbnail" {
		if !file.HasPreview() {
			s.failure(w, storage.ErrNotFound)
			return
		}
		contentType, disposition = file.MIMEType, "inline"
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": file.Name}))
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.ServeContent(w, r, file.Name, time.UnixMilli(file.CreatedAt), content)
}

func (s *API) acquireFileRequest(w http.ResponseWriter) bool {
	select {
	case s.fileRequests <- struct{}{}:
		return true
	default:
		w.Header().Set("Retry-After", "2")
		fail(w, http.StatusTooManyRequests, "tms.chat.files_busy", "Too many concurrent attachment requests")
		return false
	}
}

func (s *API) uploadFiles(w http.ResponseWriter, r *http.Request, u *storage.User) {
	if !s.chat.AttachmentsEnabled() {
		fail(w, http.StatusServiceUnavailable, "tms.chat.files_unavailable", "Attachment storage is unavailable")
		return
	}
	select {
	case s.fileUploads <- struct{}{}:
		defer func() { <-s.fileUploads }()
	default:
		w.Header().Set("Retry-After", "2")
		fail(w, http.StatusTooManyRequests, "tms.chat.files_busy", "Too many concurrent attachment uploads")
		return
	}
	if err := s.chat.CheckAttachmentSpace(); err != nil {
		s.uploadError(w, err)
		return
	}
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(30 * time.Minute))
	defer controller.SetReadDeadline(time.Time{})
	r.Body = http.MaxBytesReader(w, r.Body, chat.MaxAttachmentBytes+(64<<10))
	channel := r.URL.Query().Get("channel_id")
	clientIDs := []string{}
	uploads := []chat.StagedUpload{}
	defer func() {
		for _, upload := range uploads {
			upload.Discard()
		}
	}()
	var total int64
	stage := func(name string, data io.Reader) error {
		if len(uploads) >= chat.MaxFilesPerPost {
			return chat.ErrInvalid
		}
		upload, err := s.chat.StageAttachment(name, data, chat.MaxAttachmentBytes-total)
		if err != nil {
			return err
		}
		total += upload.Size()
		uploads = append(uploads, upload)
		return nil
	}

	if channel != "" {
		if _, err := s.chat.Channel(r.Context(), u, channel); err != nil {
			s.failure(w, err)
			return
		}
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil && r.Header.Get("Content-Type") != "" {
		s.failure(w, chat.ErrInvalid)
		return
	}
	if mediaType == "multipart/form-data" {
		form, err := r.MultipartReader()
		if err != nil {
			s.failure(w, chat.ErrInvalid)
			return
		}
		for parts := 0; ; parts++ {
			part, err := form.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				s.uploadError(w, err)
				return
			}
			if parts >= 2*chat.MaxFilesPerPost+1 {
				s.failure(w, chat.ErrInvalid)
				return
			}
			switch part.FormName() {
			case "files":
				if err := stage(part.FileName(), part); err != nil {
					s.uploadError(w, err)
					return
				}
			case "channel_id", "client_ids":
				raw, err := io.ReadAll(io.LimitReader(part, 129))
				if err != nil {
					s.uploadError(w, err)
					return
				}
				if len(raw) == 0 || len(raw) > 128 {
					s.failure(w, chat.ErrInvalid)
					return
				}
				value := string(raw)
				if part.FormName() == "client_ids" {
					clientIDs = append(clientIDs, value)
					continue
				}
				if channel != "" && channel != value {
					s.failure(w, chat.ErrInvalid)
					return
				}
				channel = value
				if _, err := s.chat.Channel(r.Context(), u, channel); err != nil {
					s.failure(w, err)
					return
				}
			default:
				s.failure(w, chat.ErrInvalid)
				return
			}
		}
	} else {
		if channel == "" {
			s.failure(w, chat.ErrInvalid)
			return
		}
		if r.ContentLength > chat.MaxAttachmentBytes {
			s.uploadError(w, chat.ErrAttachmentTooLarge)
			return
		}
		if id := r.URL.Query().Get("client_id"); id != "" {
			if len(id) > 128 {
				s.failure(w, chat.ErrInvalid)
				return
			}
			clientIDs = append(clientIDs, id)
		}
		if err := stage(r.URL.Query().Get("filename"), r.Body); err != nil {
			s.uploadError(w, err)
			return
		}
	}
	if channel == "" || (len(clientIDs) != 0 && len(clientIDs) != len(uploads)) {
		s.failure(w, chat.ErrInvalid)
		return
	}
	files, err := s.chat.UploadAttachments(r.Context(), u, channel, uploads)
	if err != nil {
		s.uploadError(w, err)
		return
	}
	write(w, http.StatusCreated, map[string]any{"file_infos": fileModels(files), "client_ids": clientIDs})
}

func (s *API) uploadError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge), errors.Is(err, chat.ErrAttachmentTooLarge):
		fail(w, http.StatusRequestEntityTooLarge, "api.file.upload_file.too_large.app_error", "Attachment upload limit is 1 GiB")
	case errors.Is(err, chat.ErrAttachmentStorageFull), errors.Is(err, syscall.ENOSPC):
		fail(w, http.StatusInsufficientStorage, "tms.chat.files_quota", "Chat attachment storage is full")
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, multipart.ErrMessageTooLarge):
		s.failure(w, chat.ErrInvalid)
	default:
		s.failure(w, err)
	}
}
