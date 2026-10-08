package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleMail(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/mail")
	path = strings.Trim(path, "/")
	parts := splitPath(path)

	switch {
	case len(parts) == 1 && parts[0] == "mailboxes" && r.Method == http.MethodPost:
		s.mailCreateFolder(w, r, au)
	case len(parts) == 2 && parts[0] == "mailboxes" && parts[1] == "order" && r.Method == http.MethodPut:
		s.mailFolderOrder(w, r, au)
	case len(parts) == 2 && parts[0] == "mailboxes" && (r.Method == http.MethodPatch || r.Method == http.MethodDelete):
		s.mailChangeFolder(w, r, au, parts[1])
	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "mailboxes":
		s.mailListMailboxes(w, r, au)
	case len(parts) >= 3 && parts[0] == "mailboxes" && parts[2] == "acl":
		s.handleMailboxACL(w, r, au, parts[1])
	case r.Method == http.MethodGet && len(parts) == 3 && parts[0] == "mailboxes" && parts[2] == "messages":
		s.mailListMessages(w, r, au, parts[1])
	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "search":
		s.mailSearch(w, r, au)
	case r.Method == http.MethodGet && len(parts) == 2 && parts[0] == "messages":
		s.mailGetMessage(w, r, au, parts[1])
	case r.Method == http.MethodPatch && len(parts) == 2 && parts[0] == "messages":
		s.mailPatchMessage(w, r, au, parts[1])
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "messages" && parts[2] == "move":
		s.mailMoveMessage(w, r, au, parts[1])
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "messages" && parts[2] == "archive":
		s.mailArchiveMessage(w, r, au, parts[1])
	case r.Method == http.MethodDelete && len(parts) == 2 && parts[0] == "messages":
		s.mailDeleteMessage(w, r, au, parts[1])
	case r.Method == http.MethodPost && len(parts) == 1 && parts[0] == "send":
		s.mailSend(w, r, au)
	case parts[0] == "vacation":
		s.handleVacation(w, r)
	case parts[0] == "delegates":
		s.handleDelegates(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(path, "/") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *Server) mailListMailboxes(w http.ResponseWriter, r *http.Request, au *authUser) {
	ctx := r.Context()
	root := s.ms.UserRoot(au.Email)
	if _, err := s.store.EnsureMailbox(ctx, au.ID, "INBOX", root); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for _, name := range []string{"Sent", "Drafts", "Trash", "Junk", "Archive"} {
		p, err := s.ms.EnsureFolder(au.Email, name)
		if err != nil {
			continue
		}
		_, _ = s.store.EnsureMailbox(ctx, au.ID, name, p)
	}
	mbs, err := s.store.ListMailboxes(ctx, au.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(mbs))
	seen := map[string]struct{}{}
	var countErr error
	appendMB := func(mb *storage.Mailbox, shared bool) {
		if _, ok := seen[mb.ID]; ok {
			return
		}
		seen[mb.ID] = struct{}{}
		msgs, err := s.store.ListMessages(ctx, mb.ID)
		if err != nil {
			countErr = err
			return
		}
		unread := 0
		for _, m := range msgs {
			if !strings.Contains(strings.ToUpper(m.Flags), `\SEEN`) {
				unread++
			}
		}
		name := mb.Name
		if shared {
			name = mb.Name + " (shared)"
		}
		out = append(out, map[string]any{
			"id": mb.ID, "name": name, "messages": len(msgs), "unread": unread, "shared": shared, "owner_id": mb.UserID, "role": storage.SystemMailboxRole(mb.Name), "system": storage.SystemMailboxRole(mb.Name) != "",
		})
	}
	for _, mb := range mbs {
		appendMB(mb, false)
	}
	if shared, err := s.store.ListSharedMailboxes(ctx, au.ID); err == nil {
		for _, mb := range shared {
			appendMB(mb, true)
		}
	}
	if countErr != nil {
		writeJSON(w, 500, map[string]string{"error": countErr.Error()})
		return
	}
	var order []string
	if raw, ok, err := s.store.GetSetting(ctx, "ui.mailfolders."+au.ID); err == nil && ok {
		_ = json.Unmarshal([]byte(raw), &order)
	}
	writeJSON(w, http.StatusOK, map[string]any{"mailboxes": out, "order": order})
}

func (s *Server) mailboxOwned(r *http.Request, au *authUser, mailboxID string) (*storage.Mailbox, int) {
	mb, err := s.store.GetMailboxByID(r.Context(), mailboxID)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, http.StatusNotFound
		}
		return nil, http.StatusInternalServerError
	}
	if mb.UserID == au.ID {
		return mb, 0
	}
	rights, err := s.store.MailboxRightsForUser(r.Context(), mailboxID, au.ID)
	if err != nil || rights == "" || !strings.Contains(rights, "r") {
		return nil, http.StatusForbidden
	}
	return mb, 0
}

func (s *Server) messageOwned(r *http.Request, au *authUser, messageID string) (*storage.Message, *storage.Mailbox, int) {
	msg, err := s.store.GetMessageByID(r.Context(), messageID)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, nil, http.StatusNotFound
		}
		return nil, nil, http.StatusInternalServerError
	}
	mb, code := s.mailboxOwned(r, au, msg.MailboxID)
	if code != 0 {
		return nil, nil, code
	}
	return msg, mb, 0
}

func (s *Server) mailListMessages(w http.ResponseWriter, r *http.Request, au *authUser, mailboxID string) {
	mb, code := s.mailboxOwned(r, au, mailboxID)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": http.StatusText(code)})
		return
	}
	msgs, err := s.store.ListMessages(r.Context(), mb.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	limit, offset := 100, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	// newest first
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	if offset > len(msgs) {
		offset = len(msgs)
	}
	end := offset + limit
	if end > len(msgs) {
		end = len(msgs)
	}
	slice := msgs[offset:end]
	out := make([]map[string]any, 0, len(slice))
	for _, m := range slice {
		subject, from, to, date := m.Subject, m.FromAddr, m.ToAddr, m.DateHdr
		if subject == "" && from == "" {
			hdr := s.parseMsgHeaders(m.FilePath)
			subject, from, to, date = hdr.Subject, hdr.From, hdr.To, hdr.Date
			if subject != "" || from != "" {
				_ = s.store.UpdateMessageHeaders(r.Context(), m.ID, subject, from, to, date)
				raw, _ := s.ms.Read(m.FilePath)
				if len(raw) > 0 {
					_ = mailsearch.Index(r.Context(), s.store, m.ID, raw)
				}
			}
		}
		if subject == "" {
			subject = "(no subject)"
		}
		out = append(out, map[string]any{
			"id": m.ID, "uid": m.UID, "mailbox_id": m.MailboxID,
			"size": m.Size, "flags": m.Flags,
			"internal_date": m.InternalDate.UTC().Format(time.RFC3339),
			"message_id":    m.MessageID,
			"subject":       subject,
			"from":          from,
			"to":            to,
			"date":          date,
			"archived":      m.Archived,
			"seen":          strings.Contains(strings.ToUpper(m.Flags), `\SEEN`),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"messages": out, "total": len(msgs), "offset": offset, "limit": limit,
	})
}

func (s *Server) mailGetMessage(w http.ResponseWriter, r *http.Request, au *authUser, messageID string) {
	msg, _, code := s.messageOwned(r, au, messageID)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": http.StatusText(code)})
		return
	}
	raw, err := s.ms.Read(msg.FilePath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "read failed"})
		return
	}
	parsed := parseMIMEMessage(raw)
	// mark seen
	if !strings.Contains(strings.ToUpper(msg.Flags), `\SEEN`) {
		flags := storage.NormalizeFlags(append(storage.ParseFlags(msg.Flags), `\Seen`))
		_ = s.store.UpdateMessageFlags(r.Context(), msg.ID, flags)
		msg.Flags = flags
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": msg.ID, "mailbox_id": msg.MailboxID, "uid": msg.UID,
		"size": msg.Size, "flags": msg.Flags,
		"internal_date": msg.InternalDate.UTC().Format(time.RFC3339),
		"message_id":    msg.MessageID,
		"subject":       parsed.Subject,
		"from":          parsed.From,
		"to":            parsed.To,
		"cc":            parsed.Cc,
		"date":          parsed.Date,
		"text":          parsed.Text,
		"html":          parsed.HTML,
	})
}

func (s *Server) mailPatchMessage(w http.ResponseWriter, r *http.Request, au *authUser, messageID string) {
	msg, _, code := s.messageOwned(r, au, messageID)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": http.StatusText(code)})
		return
	}
	var req struct {
		Flags   *[]string `json:"flags"`
		Add     []string  `json:"add_flags"`
		Remove  []string  `json:"remove_flags"`
		Seen    *bool     `json:"seen"`
		Flagged *bool     `json:"flagged"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	flags := storage.ParseFlags(msg.Flags)
	if req.Flags != nil {
		flags = *req.Flags
	}
	for _, f := range req.Add {
		flags = append(flags, f)
	}
	for _, rem := range req.Remove {
		flags = removeFlag(flags, rem)
	}
	if req.Seen != nil {
		flags = removeFlag(flags, `\Seen`)
		if *req.Seen {
			flags = append(flags, `\Seen`)
		}
	}
	if req.Flagged != nil {
		flags = removeFlag(flags, `\Flagged`)
		if *req.Flagged {
			flags = append(flags, `\Flagged`)
		}
	}
	flagStr := storage.NormalizeFlags(flags)
	if err := s.store.UpdateMessageFlags(r.Context(), msg.ID, flagStr); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": msg.ID, "flags": flagStr})
}

func removeFlag(flags []string, want string) []string {
	want = strings.ToUpper(want)
	var out []string
	for _, f := range flags {
		if strings.ToUpper(f) != want {
			out = append(out, f)
		}
	}
	return out
}

func (s *Server) mailMoveMessage(w http.ResponseWriter, r *http.Request, au *authUser, messageID string) {
	msg, _, code := s.messageOwned(r, au, messageID)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": http.StatusText(code)})
		return
	}
	var req struct {
		MailboxID string `json:"mailbox_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MailboxID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mailbox_id required"})
		return
	}
	dst, code := s.mailboxOwned(r, au, req.MailboxID)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": http.StatusText(code)})
		return
	}
	if strings.EqualFold(dst.Name, "Archive") {
		if err := s.doArchiveMessage(r.Context(), au, msg, dst); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		moved, _ := s.store.GetMessageByID(r.Context(), msg.ID)
		writeJSON(w, http.StatusOK, map[string]any{"id": moved.ID, "mailbox_id": moved.MailboxID, "uid": moved.UID, "archived": true})
		return
	}
	if msg.Archived {
		raw, err := s.ms.Read(msg.FilePath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "read failed"})
			return
		}
		newRel, size, _, err := s.ms.UnarchiveToFolder(au.Email, msg.FilePath, dst.Name)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		doc := mailsearch.ParseDocument(raw)
		_ = s.store.UpdateMessageArchiveMeta(r.Context(), msg.ID, newRel, size, false, doc.Subject, doc.From, doc.To, doc.Date)
		moved, err := s.store.MoveMessage(r.Context(), msg.ID, dst.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = mailsearch.Index(r.Context(), s.store, msg.ID, raw)
		writeJSON(w, http.StatusOK, map[string]any{"id": moved.ID, "mailbox_id": moved.MailboxID, "uid": moved.UID, "archived": false})
		return
	}
	moved, err := s.store.MoveMessage(r.Context(), msg.ID, dst.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": moved.ID, "mailbox_id": moved.MailboxID, "uid": moved.UID})
}

func (s *Server) mailArchiveMessage(w http.ResponseWriter, r *http.Request, au *authUser, messageID string) {
	msg, _, code := s.messageOwned(r, au, messageID)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": http.StatusText(code)})
		return
	}
	archPath, _ := s.ms.EnsureFolder(au.Email, "Archive")
	archMB, err := s.store.EnsureMailbox(r.Context(), au.ID, "Archive", archPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := s.doArchiveMessage(r.Context(), au, msg, archMB); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	moved, _ := s.store.GetMessageByID(r.Context(), msg.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": moved.ID, "mailbox_id": moved.MailboxID, "uid": moved.UID, "archived": true, "size": moved.Size,
	})
}

func (s *Server) doArchiveMessage(ctx context.Context, au *authUser, msg *storage.Message, archMB *storage.Mailbox) error {
	newRel, size, raw, err := s.ms.MoveToArchive(au.Email, msg.FilePath)
	if err != nil {
		return err
	}
	doc := mailsearch.ParseDocument(raw)
	if _, err := s.store.MoveMessage(ctx, msg.ID, archMB.ID); err != nil {
		return err
	}
	if err := s.store.UpdateMessageArchiveMeta(ctx, msg.ID, newRel, size, true, doc.Subject, doc.From, doc.To, doc.Date); err != nil {
		return err
	}
	return mailsearch.Index(ctx, s.store, msg.ID, raw)
}

func (s *Server) mailSearch(w http.ResponseWriter, r *http.Request, au *authUser) {
	q := r.URL.Query().Get("q")
	mailboxID := r.URL.Query().Get("mailbox_id")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if mailboxID != "" {
		if _, code := s.mailboxOwned(r, au, mailboxID); code != 0 {
			writeJSON(w, code, map[string]string{"error": http.StatusText(code)})
			return
		}
	}
	pq := mailsearch.ParseQuery(q)
	hits, err := s.store.SearchMessages(r.Context(), au.ID, mailboxID, pq.From, pq.To, pq.Subject, pq.FTSQuery(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		m := h.Message
		subj := m.Subject
		if subj == "" {
			subj = "(no subject)"
		}
		out = append(out, map[string]any{
			"id": m.ID, "uid": m.UID, "mailbox_id": m.MailboxID,
			"mailbox_name": h.MailboxName,
			"size":         m.Size, "flags": m.Flags,
			"internal_date": m.InternalDate.UTC().Format(time.RFC3339),
			"message_id":    m.MessageID,
			"subject":       subj,
			"from":          m.FromAddr,
			"to":            m.ToAddr,
			"date":          m.DateHdr,
			"archived":      m.Archived,
			"snippet":       h.Snippet,
			"rank":          h.Rank,
			"seen":          strings.Contains(strings.ToUpper(m.Flags), `\SEEN`),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": out, "q": q, "total": len(out)})
}

func (s *Server) mailDeleteMessage(w http.ResponseWriter, r *http.Request, au *authUser, messageID string) {
	msg, mb, code := s.messageOwned(r, au, messageID)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": http.StatusText(code)})
		return
	}
	// Prefer move to Trash unless already there.
	if !strings.EqualFold(mb.Name, "Trash") {
		root := s.ms.UserRoot(au.Email)
		trashPath, _ := s.ms.EnsureFolder(au.Email, "Trash")
		trash, err := s.store.EnsureMailbox(r.Context(), au.ID, "Trash", trashPath)
		if err == nil && trash.ID != mb.ID {
			if _, err := s.store.MoveMessage(r.Context(), msg.ID, trash.ID); err == nil {
				writeJSON(w, http.StatusOK, map[string]string{"status": "trashed"})
				return
			}
		}
		_ = root
	}
	if err := s.store.DeleteMessage(r.Context(), msg.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if msg.FilePath != "" {
		_ = s.ms.Delete(msg.FilePath)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) mailSend(w http.ResponseWriter, r *http.Request, au *authUser) {
	var req struct {
		To      []string `json:"to"`
		Cc      []string `json:"cc"`
		Bcc     []string `json:"bcc"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
		HTML    string   `json:"html"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	to := normalizeAddrs(req.To)
	cc := normalizeAddrs(req.Cc)
	bcc := normalizeAddrs(req.Bcc)
	if len(to) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "to required"})
		return
	}
	user, err := s.store.GetUserByID(r.Context(), au.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
		return
	}
	msgid := fmt.Sprintf("<%d.%s@tayga>", time.Now().UnixNano(), strings.ReplaceAll(au.ID, "-", "")[:8])
	raw := buildOutgoingMIME(user.Email, user.DisplayName, to, cc, bcc, req.Subject, req.Text, req.HTML, msgid)

	// Save copy to Sent
	sentPath, _ := s.ms.EnsureFolder(user.Email, "Sent")
	sentMB, err := s.store.EnsureMailbox(r.Context(), user.ID, "Sent", sentPath)
	if err == nil {
		if rel, size, derr := s.ms.Deliver(user.Email, "Sent", raw); derr == nil {
			sm := &storage.Message{
				MailboxID: sentMB.ID, Size: size, Flags: `\Seen`,
				InternalDate: time.Now().UTC(), FilePath: rel, MessageID: msgid,
			}
			mailsearch.ApplyHeaders(sm, raw)
			if inserted, ierr := s.store.InsertMessage(r.Context(), sm); ierr == nil {
				_ = mailsearch.Index(r.Context(), s.store, inserted.ID, raw)
			}
		}
	}

	all := append(append([]string{}, to...), cc...)
	all = append(all, bcc...)
	var local, remote []string
	for _, addr := range all {
		if _, err := s.store.ResolveRecipient(r.Context(), addr); err == nil {
			local = append(local, addr)
		} else {
			remote = append(remote, addr)
		}
	}
	for _, rcpt := range local {
		ru, err := s.store.ResolveRecipient(r.Context(), rcpt)
		if err != nil {
			s.writeMailLog(r.Context(), user, "failed", "inbound", rcpt, msgid, int64(len(raw)), err.Error())
			continue
		}
		root := s.ms.UserRoot(ru.Email)
		mb, err := s.store.EnsureMailbox(r.Context(), ru.ID, "INBOX", root)
		if err != nil {
			s.writeMailLog(r.Context(), user, "failed", "inbound", rcpt, msgid, int64(len(raw)), err.Error())
			continue
		}
		if rel, size, derr := s.ms.Deliver(ru.Email, "INBOX", raw); derr == nil {
			im := &storage.Message{
				MailboxID: mb.ID, Size: size, Flags: "",
				InternalDate: time.Now().UTC(), FilePath: rel, MessageID: msgid,
			}
			mailsearch.ApplyHeaders(im, raw)
			if inserted, ierr := s.store.InsertMessage(r.Context(), im); ierr == nil {
				_ = mailsearch.Index(r.Context(), s.store, inserted.ID, raw)
			}
			s.writeMailLog(r.Context(), user, "delivered", "inbound", rcpt, msgid, size, "web")
		} else {
			s.writeMailLog(r.Context(), user, "failed", "inbound", rcpt, msgid, int64(len(raw)), derr.Error())
		}
	}
	for _, rcpt := range remote {
		_, err := s.store.EnqueueOutbound(r.Context(), &storage.OutboundItem{
			EnvelopeFrom: user.Email,
			EnvelopeTo:   rcpt,
			MessageID:    msgid,
			Data:         raw,
			MaxAttempts:  8,
		})
		if err != nil {
			s.writeMailLog(r.Context(), user, "failed", "outbound", rcpt, msgid, int64(len(raw)), err.Error())
			continue
		}
		s.writeMailLog(r.Context(), user, "queued", "outbound", rcpt, msgid, int64(len(raw)), "web")
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "accepted", "message_id": msgid,
		"local": len(local), "remote": len(remote),
	})
}

// writeMailLog records a searchable admin mail-log line (best-effort).
func (s *Server) writeMailLog(ctx context.Context, user *storage.User, event, direction, rcpt, msgid string, size int64, detail string) {
	if s == nil || s.store == nil {
		return
	}
	from := ""
	tenantID := ""
	if user != nil {
		from = strings.ToLower(strings.TrimSpace(user.Email))
		tenantID = user.TenantID
	}
	domain := storage.DomainOfEmail(rcpt)
	if direction == "outbound" {
		domain = storage.DomainOfEmail(from)
	}
	if tenantID == "" && domain != "" {
		if d, err := s.store.GetDomainByName(ctx, domain); err == nil && d != nil {
			tenantID = d.TenantID
		}
	}
	if len(detail) > 500 {
		detail = detail[:500]
	}
	e := &storage.MailLogEntry{
		TenantID:  tenantID,
		Domain:    domain,
		Event:     event,
		Direction: direction,
		Peer:      "web",
		MailFrom:  from,
		RcptTo:    strings.ToLower(strings.TrimSpace(rcpt)),
		MessageID: msgid,
		Size:      size,
		Detail:    detail,
	}
	if s.siem != nil {
		s.siem.EmitMail(e.Event, e.Direction, e.Peer, e.MailFrom, e.RcptTo, e.MessageID, e.Detail, e.Size)
	}
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.store.InsertMailLog(c, e); err != nil && s.log != nil {
			s.log.Warn("mail log write failed", "err", err)
		}
	}()
}

type msgHdr struct {
	Subject, From, To, Cc, Date string
}

func (s *Server) parseMsgHeaders(filePath string) msgHdr {
	h := msgHdr{Subject: "(no subject)"}
	if s.ms == nil || filePath == "" {
		return h
	}
	data, err := s.ms.Read(filePath)
	if err != nil {
		return h
	}
	m, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return h
	}
	if v := strings.TrimSpace(m.Header.Get("Subject")); v != "" {
		h.Subject = v
	}
	h.From = m.Header.Get("From")
	h.To = m.Header.Get("To")
	h.Cc = m.Header.Get("Cc")
	h.Date = m.Header.Get("Date")
	return h
}

type parsedMsg struct {
	Subject, From, To, Cc, Date, Text, HTML string
}

func parseMIMEMessage(raw []byte) parsedMsg {
	out := parsedMsg{Subject: "(no subject)"}
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		out.Text = string(raw)
		return out
	}
	if v := strings.TrimSpace(m.Header.Get("Subject")); v != "" {
		out.Subject = v
	}
	out.From = m.Header.Get("From")
	out.To = m.Header.Get("To")
	out.Cc = m.Header.Get("Cc")
	out.Date = m.Header.Get("Date")
	ct := m.Header.Get("Content-Type")
	media, params, _ := mime.ParseMediaType(ct)
	if strings.HasPrefix(media, "multipart/") {
		mr := multipart.NewReader(m.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			body, _ := io.ReadAll(io.LimitReader(p, 2<<20))
			pct, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
			switch {
			case pct == "text/plain" && out.Text == "":
				out.Text = string(body)
			case pct == "text/html" && out.HTML == "":
				out.HTML = string(body)
			}
		}
	} else {
		body, _ := io.ReadAll(io.LimitReader(m.Body, 2<<20))
		if media == "text/html" {
			out.HTML = string(body)
		} else {
			out.Text = string(body)
		}
	}
	return out
}

func normalizeAddrs(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, a := range in {
		a = strings.ToLower(strings.TrimSpace(a))
		a = strings.Trim(a, "<>")
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	return out
}

func buildOutgoingMIME(fromEmail, fromName string, to, cc, bcc []string, subject, text, html, msgid string) []byte {
	var b strings.Builder
	from := fromEmail
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", fromName, fromEmail)
	}
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	if len(cc) > 0 {
		fmt.Fprintf(&b, "Cc: %s\r\n", strings.Join(cc, ", "))
	}
	if len(bcc) > 0 {
		fmt.Fprintf(&b, "Bcc: %s\r\n", strings.Join(bcc, ", "))
	}
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: %s\r\n", msgid)
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	if html != "" && text != "" {
		boundary := "tayga-" + strconv.FormatInt(time.Now().UnixNano(), 36)
		fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary)
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", boundary, text)
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s\r\n", boundary, html)
		fmt.Fprintf(&b, "--%s--\r\n", boundary)
	} else if html != "" {
		b.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
		b.WriteString(html)
	} else {
		b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
		b.WriteString(text)
	}
	return []byte(b.String())
}
