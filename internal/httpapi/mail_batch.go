package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

// Serialize overlapping web operations, but never hold a database transaction
// while compressing an attachment. Storage also detects moves by other protocols.
func (s *Server) lockMailMessage(id string) func() {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	m := &s.mailMutationLocks[h.Sum32()%uint32(len(s.mailMutationLocks))]
	m.Lock()
	return m.Unlock
}

func (s *Server) ensureMailFolder(ctx context.Context, au *authUser, name string) (*storage.Mailbox, error) {
	path, err := s.ms.EnsureFolder(au.Email, name)
	if err != nil {
		return nil, err
	}
	return s.store.EnsureMailbox(ctx, au.ID, name, path)
}

func (s *Server) moveWebMessage(ctx context.Context, au *authUser, msg *storage.Message, dst *storage.Mailbox) (*storage.Message, error) {
	archive := strings.EqualFold(dst.Name, "Archive")
	if msg.MailboxID == dst.ID {
		return msg, nil
	}
	if archive == msg.Archived {
		return s.store.MoveMessageContent(ctx, msg, dst.ID, msg.FilePath, msg.Size, msg.Archived)
	}
	raw, err := s.ms.Read(msg.FilePath)
	if err != nil {
		return nil, err
	}
	var path string
	var size int64
	if archive {
		path, size, err = s.ms.DeliverGzip(au.Email, "Archive", raw)
	} else {
		path, size, err = s.ms.Deliver(au.Email, dst.Name, raw)
	}
	if err != nil {
		return nil, err
	}
	moved, err := s.store.MoveMessageContent(ctx, msg, dst.ID, path, size, archive)
	if err != nil {
		// A lost/cancelled commit response can be ambiguous. Never remove content
		// unless a fresh database read proves the candidate is not referenced.
		checkCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		current, lookupErr := s.store.GetMessageByID(checkCtx, msg.ID)
		if lookupErr == storage.ErrNotFound || lookupErr == nil && current.FilePath != path {
			_ = s.ms.Delete(path)
		}
		return nil, err
	}
	if err := s.ms.Delete(msg.FilePath); err != nil {
		s.log.Warn("mail old content cleanup failed", "message_id", msg.ID, "error", err)
	}
	// Compression and folder changes do not alter RFC822 content. Keep the
	// existing headers/search index instead of reparsing and reindexing it.
	return moved, nil
}

func (s *Server) deleteWebMessage(ctx context.Context, au *authUser, msg *storage.Message, mb *storage.Mailbox) (string, error) {
	if !strings.EqualFold(mb.Name, "Trash") {
		trash, err := s.ensureMailFolder(ctx, au, "Trash")
		if err != nil {
			return "", err
		}
		_, err = s.store.MoveMessageContent(ctx, msg, trash.ID, msg.FilePath, msg.Size, msg.Archived)
		return "trashed", err // A failed move must never fall through to permanent deletion.
	}
	if err := s.store.DeleteMessage(ctx, msg.ID); err != nil {
		return "", err
	}
	if err := s.ms.Delete(msg.FilePath); err != nil {
		s.log.Warn("mail deleted content cleanup failed", "message_id", msg.ID, "error", err)
	}
	return "deleted", nil
}

func mailError(err error) (int, string) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return 404, "message_not_found"
	case errors.Is(err, storage.ErrMessageChanged):
		return 409, "message_changed"
	case errors.Is(err, os.ErrNotExist):
		return 409, "message_content_unavailable"
	default:
		return 503, "mail_operation_failed"
	}
}
func (s *Server) mailMutationError(w http.ResponseWriter, id string, err error) {
	s.log.Error("mail operation failed", "message_id", id, "error", err)
	code, message := mailError(err)
	writeJSON(w, code, map[string]string{"error": message})
}

type mailBatchItem struct {
	ID string `json:"id"`
	// The mailbox observed by the UI makes retries safe: a second delete must
	// not permanently remove a message just moved to Trash by the first request.
	MailboxID string `json:"mailbox_id"`
}
type mailBatchResult struct {
	ID    string `json:"id"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (s *Server) mailBatch(w http.ResponseWriter, r *http.Request, au *authUser) {
	var req struct {
		Action string          `json:"action"`
		Items  []mailBatchItem `json:"items"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&req); err != nil || len(req.Items) == 0 || len(req.Items) > 50 {
		writeJSON(w, 400, map[string]string{"error": "invalid batch"})
		return
	}
	switch req.Action {
	case "read", "unread", "archive", "spam", "delete":
	default:
		writeJSON(w, 400, map[string]string{"error": "invalid action"})
		return
	}
	seen := map[string]bool{}
	for _, item := range req.Items {
		if item.ID == "" || item.MailboxID == "" || len(item.ID) > 128 || len(item.MailboxID) > 128 || seen[item.ID] {
			writeJSON(w, 400, map[string]string{"error": "invalid items"})
			return
		}
		seen[item.ID] = true
	}
	results := make([]mailBatchResult, 0, len(req.Items))
	var dst *storage.Mailbox
	for _, item := range req.Items {
		result := mailBatchResult{ID: item.ID}
		func() {
			unlock := s.lockMailMessage(item.ID)
			defer unlock()
			msg, mb, code := s.messageOwned(r, au, item.ID)
			if code != 0 || mb.UserID != au.ID {
				result.Error = "message_not_found"
				return
			}
			if msg.MailboxID != item.MailboxID {
				result.Error = "message_changed"
				return
			}
			var err error
			switch req.Action {
			case "read", "unread":
				flags := removeFlag(storage.ParseFlags(msg.Flags), `\Seen`)
				if req.Action == "read" {
					flags = append(flags, `\Seen`)
				}
				normalized := storage.NormalizeFlags(flags)
				if normalized != msg.Flags {
					err = s.store.UpdateMessageFlags(r.Context(), msg.ID, normalized)
				}
			case "archive", "spam":
				if dst == nil {
					folder := "Archive"
					if req.Action == "spam" {
						folder = "Junk"
					}
					dst, err = s.ensureMailFolder(r.Context(), au, folder)
				}
				if err == nil {
					_, err = s.moveWebMessage(r.Context(), au, msg, dst)
				}
			case "delete":
				_, err = s.deleteWebMessage(r.Context(), au, msg, mb)
			}
			if err != nil {
				_, result.Error = mailError(err)
				s.log.Error("mail batch item failed", "action", req.Action, "message_id", item.ID, "error", err)
				return
			}
			result.OK = true
		}()
		results = append(results, result)
	}
	writeJSON(w, 200, map[string]any{"results": results})
}
