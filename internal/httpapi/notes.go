package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tayga/tms/internal/noteutil"
	"github.com/tayga/tms/internal/storage"
)

const maxNoteAttachmentBytes = 32 << 20

func (s *Server) notesRoot() (string, error) {
	if s.ms == nil {
		return "", errString("mailstore unavailable")
	}
	root := filepath.Join(s.ms.Root, "notes")
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", err
	}
	return root, nil
}

func noteFolderJSON(f *storage.NoteFolder, ownerID, rights string) map[string]any {
	shared := ownerID != "" && f.UserID != ownerID
	return map[string]any{
		"id": f.ID, "name": f.Name, "display_name": f.DisplayName,
		"parent_id": f.ParentID, "position": f.Position, "ctag": f.CTag,
		"owner_id": f.UserID, "shared": shared, "rights": rights,
		"deletable": !shared && f.Name != "notes",
	}
}

func noteItemJSON(n *storage.NoteItem, atts []map[string]any, rights string) map[string]any {
	item := map[string]any{
		"id": n.ID, "folder_id": n.FolderID, "user_id": n.UserID, "title": n.Title,
		"document": n.DocumentJSON, "body_html": n.BodyHTML, "body_text": n.BodyText,
		"pinned": n.Pinned, "archived": n.Archived, "color": n.Color, "etag": n.ETag,
		"rights":     rights,
		"created_at": n.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": n.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if len(atts) > 0 {
		item["attachments"] = atts
	}
	return item
}

func (s *Server) noteCan(ctx context.Context, userID, noteID, need string) (*storage.NoteItem, string, error) {
	rights, err := s.store.NoteRightsForUser(ctx, noteID, userID)
	if err != nil {
		return nil, "", err
	}
	if need == "write" && rights != "write" {
		return nil, rights, storage.ErrNotFound
	}
	n, err := s.store.GetNoteItemByID(ctx, noteID)
	if err != nil {
		return nil, "", err
	}
	return n, rights, nil
}

func (s *Server) noteFolderCan(ctx context.Context, userID, folderID, need string) (*storage.NoteFolder, string, error) {
	rights, err := s.store.NoteFolderRightsForUser(ctx, folderID, userID)
	if err != nil {
		return nil, "", err
	}
	if need == "write" && rights != "write" {
		return nil, rights, storage.ErrNotFound
	}
	f, err := s.store.GetNoteFolder(ctx, folderID)
	if err != nil {
		return nil, "", err
	}
	return f, rights, nil
}

func noteAttachJSON(a *storage.NoteAttachment) map[string]any {
	return map[string]any{
		"id": a.ID, "note_id": a.NoteID, "filename": a.Filename,
		"content_type": a.ContentType, "size": a.Size, "kind": a.Kind,
		"url":        "/api/v1/notes/attachments/" + a.ID,
		"created_at": a.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *Server) handleNotes(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	_ = s.store.EnsureNoteDefaults(r.Context(), au.ID)
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/notes"), "/")
	parts := splitPath(path)

	switch {
	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "folders":
		list, err := s.store.ListNoteFoldersForUser(r.Context(), au.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(list))
		for _, f := range list {
			rights := "write"
			if f.UserID != au.ID {
				if fr, err := s.store.NoteFolderRightsForUser(r.Context(), f.ID, au.ID); err == nil {
					rights = fr
				} else {
					rights = "read"
				}
			}
			out = append(out, noteFolderJSON(f, au.ID, rights))
		}
		writeJSON(w, http.StatusOK, map[string]any{"folders": out})

	case r.Method == http.MethodPost && len(parts) == 1 && parts[0] == "folders":
		var req struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			ParentID    string `json:"parent_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		display := strings.TrimSpace(req.DisplayName)
		if display == "" {
			display = strings.TrimSpace(req.Name)
		}
		if display == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "display_name required"})
			return
		}
		f, err := s.store.CreateNoteFolder(r.Context(), &storage.NoteFolder{
			UserID: au.ID, Name: req.Name, DisplayName: display, ParentID: strings.TrimSpace(req.ParentID),
		})
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, noteFolderJSON(f, au.ID, "write"))

	case r.Method == http.MethodPatch && len(parts) == 2 && parts[0] == "folders":
		f, err := s.store.GetNoteFolderByID(r.Context(), au.ID, parts[1])
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		var req struct {
			DisplayName *string `json:"display_name"`
			ParentID    *string `json:"parent_id"`
			Position    *int    `json:"position"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if req.DisplayName != nil {
			f.DisplayName = strings.TrimSpace(*req.DisplayName)
		}
		if req.ParentID != nil {
			f.ParentID = strings.TrimSpace(*req.ParentID)
		}
		if req.Position != nil {
			f.Position = *req.Position
		}
		if err := s.store.UpdateNoteFolder(r.Context(), f); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, noteFolderJSON(f, au.ID, "write"))

	case r.Method == http.MethodDelete && len(parts) == 2 && parts[0] == "folders":
		if err := s.store.DeleteNoteFolder(r.Context(), au.ID, parts[1]); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	case len(parts) == 3 && parts[0] == "folders" && parts[2] == "acl":
		s.handleNoteFolderACL(w, r, au, parts[1])

	case r.Method == http.MethodGet && (len(parts) == 0 || (len(parts) == 1 && parts[0] == "")):
		folderID := r.URL.Query().Get("folder_id")
		includeArchived := r.URL.Query().Get("archived") == "1"
		sharedOnly := r.URL.Query().Get("shared") == "1"
		var list []*storage.NoteItem
		var err error
		switch {
		case sharedOnly:
			list, err = s.store.ListSharedNoteItems(r.Context(), au.ID)
		case folderID != "":
			if _, _, ferr := s.noteFolderCan(r.Context(), au.ID, folderID, "read"); ferr != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "folder not found"})
				return
			}
			list, err = s.store.ListNoteItemsInFolder(r.Context(), folderID, includeArchived)
		default:
			list, err = s.store.ListNoteItems(r.Context(), au.ID, "", includeArchived)
			if err == nil {
				shared, serr := s.store.ListSharedNoteItems(r.Context(), au.ID)
				if serr == nil {
					seen := map[string]struct{}{}
					for _, n := range list {
						seen[n.ID] = struct{}{}
					}
					for _, n := range shared {
						if _, ok := seen[n.ID]; !ok {
							list = append(list, n)
						}
					}
				}
			}
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(list))
		for _, n := range list {
			rights := "write"
			if rgt, err := s.store.NoteRightsForUser(r.Context(), n.ID, au.ID); err == nil {
				rights = rgt
			}
			out = append(out, noteItemJSON(n, nil, rights))
		}
		writeJSON(w, http.StatusOK, map[string]any{"notes": out})

	case r.Method == http.MethodPost && len(parts) == 0:
		var req struct {
			FolderID string `json:"folder_id"`
			Title    string `json:"title"`
			Document string `json:"document"`
			BodyHTML string `json:"body_html"`
			BodyText string `json:"body_text"`
			Pinned   bool   `json:"pinned"`
			Color    string `json:"color"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		folderID := strings.TrimSpace(req.FolderID)
		if folderID == "" {
			def, err := s.store.EnsureNoteFolder(r.Context(), au.ID, "notes", "Notes")
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			folderID = def.ID
		} else if _, _, err := s.noteFolderCan(r.Context(), au.ID, folderID, "write"); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "folder not found"})
			return
		}
		docJSON, htmlOut, textOut := normalizeNoteBodies(req.Document, req.BodyHTML, req.BodyText)
		n, err := s.store.CreateNoteItem(r.Context(), &storage.NoteItem{
			FolderID: folderID, UserID: au.ID, Title: strings.TrimSpace(req.Title),
			DocumentJSON: docJSON, BodyHTML: htmlOut, BodyText: textOut,
			Pinned: req.Pinned, Color: strings.TrimSpace(req.Color),
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, noteItemJSON(n, nil, "write"))

	case len(parts) == 2 && parts[1] == "acl":
		s.handleNoteACL(w, r, au, parts[0])

	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] != "folders" && parts[0] != "attachments":
		n, rights, err := s.noteCan(r.Context(), au.ID, parts[0], "read")
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		atts := s.noteAttachmentsJSON(r, n.ID)
		writeJSON(w, http.StatusOK, noteItemJSON(n, atts, rights))

	case r.Method == http.MethodPatch && len(parts) == 1:
		n, rights, err := s.noteCan(r.Context(), au.ID, parts[0], "write")
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		var req struct {
			FolderID *string `json:"folder_id"`
			Title    *string `json:"title"`
			Document *string `json:"document"`
			BodyHTML *string `json:"body_html"`
			BodyText *string `json:"body_text"`
			Pinned   *bool   `json:"pinned"`
			Archived *bool   `json:"archived"`
			Color    *string `json:"color"`
			ETag     *string `json:"etag"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		ifMatch := strings.Trim(r.Header.Get("If-Match"), `"`)
		if ifMatch == "" && req.ETag != nil {
			ifMatch = strings.Trim(*req.ETag, `"`)
		}
		if ifMatch != "" && ifMatch != strings.Trim(n.ETag, `"`) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": "etag mismatch", "etag": n.ETag,
				"note": noteItemJSON(n, s.noteAttachmentsJSON(r, n.ID), rights),
			})
			return
		}
		if req.FolderID != nil {
			if _, _, err := s.noteFolderCan(r.Context(), au.ID, *req.FolderID, "write"); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "folder not found"})
				return
			}
			n.FolderID = *req.FolderID
		}
		if req.Title != nil {
			n.Title = strings.TrimSpace(*req.Title)
		}
		if req.Document != nil {
			n.DocumentJSON, n.BodyHTML, n.BodyText = normalizeNoteBodies(*req.Document, "", "")
		} else if req.BodyHTML != nil {
			n.DocumentJSON, n.BodyHTML, n.BodyText = normalizeNoteBodies("", *req.BodyHTML, "")
		} else if req.BodyText != nil {
			n.DocumentJSON, n.BodyHTML, n.BodyText = normalizeNoteBodies("", "", *req.BodyText)
		}

		if req.Pinned != nil {
			n.Pinned = *req.Pinned
		}
		if req.Archived != nil {
			n.Archived = *req.Archived
		}
		if req.Color != nil {
			n.Color = strings.TrimSpace(*req.Color)
		}
		if err := s.store.UpdateNoteItem(r.Context(), n); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, noteItemJSON(n, s.noteAttachmentsJSON(r, n.ID), rights))

	case r.Method == http.MethodDelete && len(parts) == 1:
		n, _, err := s.noteCan(r.Context(), au.ID, parts[0], "write")
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if err := s.store.DeleteNoteItemByID(r.Context(), n.ID); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if root, err := s.notesRoot(); err == nil {
			_ = os.RemoveAll(filepath.Join(root, n.UserID, n.ID))
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	case len(parts) == 2 && parts[1] == "attachments":
		s.handleNoteAttachments(w, r, au, parts[0])

	case len(parts) == 2 && parts[1] == "drawing":
		s.handleNoteDrawing(w, r, au, parts[0])

	case len(parts) == 2 && parts[0] == "attachments":
		s.handleNoteAttachmentByID(w, r, au, parts[1])

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func normalizeNoteBodies(document, bodyHTML, bodyText string) (docJSON, htmlOut, textOut string) {
	document = strings.TrimSpace(document)
	bodyHTML = strings.TrimSpace(bodyHTML)
	bodyText = strings.TrimSpace(bodyText)
	switch {
	case document != "" && (strings.HasPrefix(document, "{") || strings.HasPrefix(document, "[")):
		doc := noteutil.ParseDocumentJSON(document)
		docJSON = noteutil.MarshalDocumentJSON(doc)
		htmlOut, textOut = noteutil.DeriveBodies(docJSON)
	case bodyHTML != "":
		doc := noteutil.DocumentFromHTMLOrText(bodyHTML)
		docJSON = noteutil.MarshalDocumentJSON(doc)
		htmlOut, textOut = noteutil.DeriveBodies(docJSON)
	case bodyText != "":
		doc := noteutil.DocumentFromText(bodyText)
		docJSON = noteutil.MarshalDocumentJSON(doc)
		htmlOut, textOut = noteutil.DeriveBodies(docJSON)
	default:
		doc := noteutil.EmptyDoc()
		docJSON = noteutil.MarshalDocumentJSON(doc)
		htmlOut, textOut = noteutil.DeriveBodies(docJSON)
	}
	return docJSON, htmlOut, textOut
}

func (s *Server) noteAttachmentsJSON(r *http.Request, noteID string) []map[string]any {
	list, err := s.store.ListNoteAttachments(r.Context(), noteID)
	if err != nil || len(list) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, a := range list {
		out = append(out, noteAttachJSON(a))
	}
	return out
}

func (s *Server) handleNoteFolderACL(w http.ResponseWriter, r *http.Request, au *authUser, folderID string) {
	f, err := s.store.GetNoteFolderByID(r.Context(), au.ID, folderID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	_ = f
	switch {
	case r.Method == http.MethodGet:
		entries, err := s.store.ListNoteFolderACL(r.Context(), folderID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			email := ""
			if u, err := s.store.GetUserByID(r.Context(), e.GranteeUserID); err == nil {
				email = u.Email
			}
			out = append(out, map[string]any{"user_id": e.GranteeUserID, "email": email, "rights": e.Rights})
		}
		writeJSON(w, http.StatusOK, map[string]any{"acl": out})
	case r.Method == http.MethodPut:
		var req struct {
			Email  string `json:"email"`
			Rights string `json:"rights"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		u, err := s.store.GetUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		if u.ID == au.ID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot share with yourself"})
			return
		}
		rights := strings.TrimSpace(req.Rights)
		if rights == "" {
			rights = "write"
		}
		if err := s.store.SetNoteFolderACL(r.Context(), folderID, u.ID, rights); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case r.Method == http.MethodDelete:
		grantee := r.URL.Query().Get("user_id")
		if grantee == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id required"})
			return
		}
		_ = s.store.DeleteNoteFolderACL(r.Context(), folderID, grantee)
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleNoteACL(w http.ResponseWriter, r *http.Request, au *authUser, noteID string) {
	n, err := s.store.GetNoteItem(r.Context(), au.ID, noteID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	_ = n
	switch {
	case r.Method == http.MethodGet:
		entries, err := s.store.ListNoteACL(r.Context(), noteID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			email := ""
			if u, err := s.store.GetUserByID(r.Context(), e.GranteeUserID); err == nil {
				email = u.Email
			}
			out = append(out, map[string]any{"user_id": e.GranteeUserID, "email": email, "rights": e.Rights})
		}
		writeJSON(w, http.StatusOK, map[string]any{"acl": out})
	case r.Method == http.MethodPut:
		var req struct {
			Email  string `json:"email"`
			Rights string `json:"rights"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		u, err := s.store.GetUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		if u.ID == au.ID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot share with yourself"})
			return
		}
		rights := strings.TrimSpace(req.Rights)
		if rights == "" {
			rights = "write"
		}
		if err := s.store.SetNoteACL(r.Context(), noteID, u.ID, rights); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case r.Method == http.MethodDelete:
		grantee := r.URL.Query().Get("user_id")
		if grantee == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id required"})
			return
		}
		_ = s.store.DeleteNoteACL(r.Context(), noteID, grantee)
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleNoteAttachments(w http.ResponseWriter, r *http.Request, au *authUser, noteID string) {
	need := "read"
	if r.Method == http.MethodPost {
		need = "write"
	}
	n, _, err := s.noteCan(r.Context(), au.ID, noteID, need)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"attachments": s.noteAttachmentsJSON(r, n.ID)})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, maxNoteAttachmentBytes+(1<<20))
		if err := r.ParseMultipartForm(maxNoteAttachmentBytes); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "multipart form required (file)"})
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		file, hdr, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file required"})
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxNoteAttachmentBytes+1))
		if err != nil || int64(len(data)) > maxNoteAttachmentBytes {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file too large"})
			return
		}
		filename := filepath.Base(hdr.Filename)
		if filename == "" || filename == "." {
			filename = "file"
		}
		ct := hdr.Header.Get("Content-Type")
		if ct == "" {
			ct = "application/octet-stream"
		}
		kind := r.FormValue("kind")
		if kind == "" {
			kind = "file"
		}
		attID := storage.NewID()
		rel := filepath.ToSlash(filepath.Join(n.UserID, n.ID, attID+"_"+sanitizeAttachName(filename)))
		root, err := s.notesRoot()
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
			return
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if err := os.WriteFile(abs, data, 0o640); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		saved, err := s.store.CreateNoteAttachment(r.Context(), &storage.NoteAttachment{
			ID: attID, NoteID: n.ID, UserID: au.ID, Filename: filename,
			ContentType: ct, Size: int64(len(data)), StoragePath: rel, Kind: kind,
		})
		if err != nil {
			_ = os.Remove(abs)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, noteAttachJSON(saved))
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleNoteAttachmentByID(w http.ResponseWriter, r *http.Request, au *authUser, id string) {
	a, err := s.store.GetNoteAttachment(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	need := "read"
	if r.Method == http.MethodDelete {
		need = "write"
	}
	if _, _, err := s.noteCan(r.Context(), au.ID, a.NoteID, need); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		root, err := s.notesRoot()
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
			return
		}
		abs := filepath.Join(root, filepath.FromSlash(a.StoragePath))
		f, err := os.Open(abs)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "file missing"})
			return
		}
		defer f.Close()
		st, _ := f.Stat()
		ct := a.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": a.Filename}))
		http.ServeContent(w, r, a.Filename, st.ModTime(), f)
	case http.MethodDelete:
		if root, err := s.notesRoot(); err == nil {
			_ = os.Remove(filepath.Join(root, filepath.FromSlash(a.StoragePath)))
		}
		if err := s.store.DeleteNoteAttachment(r.Context(), a.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleNoteDrawing(w http.ResponseWriter, r *http.Request, au *authUser, noteID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	n, rights, err := s.noteCan(r.Context(), au.ID, noteID, "write")
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	var req struct {
		StrokesJSON string `json:"strokes"`     // raw JSON string of strokes
		PreviewPNG  string `json:"preview_png"` // data URL or base64
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	root, err := s.notesRoot()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	strokes := []byte(req.StrokesJSON)
	if len(strokes) == 0 {
		strokes = []byte("[]")
	}
	drawID := storage.NewID()
	drawRel := filepath.ToSlash(filepath.Join(n.UserID, n.ID, drawID+"_strokes.json"))
	drawAbs := filepath.Join(root, filepath.FromSlash(drawRel))
	if err := os.MkdirAll(filepath.Dir(drawAbs), 0o750); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := os.WriteFile(drawAbs, strokes, 0o640); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	drawAtt, err := s.store.CreateNoteAttachment(r.Context(), &storage.NoteAttachment{
		ID: drawID, NoteID: n.ID, UserID: au.ID, Filename: "drawing.json",
		ContentType: "application/json", Size: int64(len(strokes)), StoragePath: drawRel, Kind: "drawing",
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var previewJSON map[string]any
	if png := decodeDataURL(req.PreviewPNG); len(png) > 0 {
		prevID := storage.NewID()
		prevRel := filepath.ToSlash(filepath.Join(n.UserID, n.ID, prevID+"_preview.png"))
		prevAbs := filepath.Join(root, filepath.FromSlash(prevRel))
		if err := os.WriteFile(prevAbs, png, 0o640); err == nil {
			prevAtt, err := s.store.CreateNoteAttachment(r.Context(), &storage.NoteAttachment{
				ID: prevID, NoteID: n.ID, UserID: au.ID, Filename: "drawing.png",
				ContentType: "image/png", Size: int64(len(png)), StoragePath: prevRel, Kind: "preview",
			})
			if err == nil {
				previewJSON = noteAttachJSON(prevAtt)
			}
		}
	}
	// Append drawing node to document
	doc := noteutil.ParseDocumentJSON(n.DocumentJSON)
	attrs := map[string]any{"attachment_id": drawAtt.ID}
	if previewJSON != nil {
		attrs["preview_id"] = previewJSON["id"]
	}
	doc.Content = append(doc.Content, noteutil.Node{Type: "drawing", Attrs: attrs})
	n.DocumentJSON = noteutil.MarshalDocumentJSON(doc)
	n.BodyHTML, n.BodyText = noteutil.DeriveBodies(n.DocumentJSON)
	_ = s.store.UpdateNoteItem(r.Context(), n)

	out := map[string]any{"drawing": noteAttachJSON(drawAtt), "note": noteItemJSON(n, s.noteAttachmentsJSON(r, n.ID), rights)}
	if previewJSON != nil {
		out["preview"] = previewJSON
	}
	writeJSON(w, http.StatusCreated, out)
}

func decodeDataURL(s string) []byte {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if i := strings.Index(s, ","); i >= 0 && strings.Contains(s[:i], "base64") {
		s = s[i+1:]
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}
