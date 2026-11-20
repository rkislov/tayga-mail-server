package httpapi

import (
	"embed"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tayga/tms/internal/notify"
	"github.com/tayga/tms/internal/storage"
)

//go:embed file_request.html file_request.js file_request.css
var fileRequestAssets embed.FS
var fileRequestPage = template.Must(template.ParseFS(fileRequestAssets, "file_request.html"))

func (s *Server) handleFileRequestAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(405)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/file-request-assets/")
	if name != "file_request.js" && name != "file_request.css" {
		http.NotFound(w, r)
		return
	}
	data, _ := fileRequestAssets.ReadFile(name)
	contentType := "text/javascript; charset=utf-8"
	if strings.HasSuffix(name, ".css") {
		contentType = "text/css; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache")
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}
func fileRequestName(name string) bool {
	if name == "" || len(name) > 180 || !utf8.ValidString(name) || strings.TrimSpace(name) != name || name == "." || name == ".." || strings.HasPrefix(name, ".upload-") || strings.ContainsAny(name, `/\:`) {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// Caller holds fileMu, shared with link revocation, moves and managed writes.
func (s *Server) validateFileRequest(r *http.Request, initial *storage.FileShare) (*storage.FileShare, os.FileInfo, error) {
	sh, err := s.store.GetFileShareByToken(r.Context(), initial.Token)
	if err != nil || sh.ID != initial.ID || sh.UserID != initial.UserID || sh.Rights != "upload" || sh.Path != initial.Path || sh.ExpiresAt == nil || !time.Now().Before(*sh.ExpiresAt) {
		return nil, nil, os.ErrPermission
	}
	if !s.fileOwnerActive(r.Context(), sh.UserID) {
		return nil, nil, os.ErrPermission
	}
	root, err := s.filesRoot(sh.UserID)
	if err != nil {
		return nil, nil, err
	}
	abs, err := safeJoin(root, sh.Path)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return nil, nil, os.ErrPermission
	}
	return sh, info, nil
}
func (s *Server) handleFileRequest(w http.ResponseWriter, r *http.Request, initial *storage.FileShare) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT")
		w.WriteHeader(405)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	// An upload capability must never turn into a read/path-probing capability.
	for key := range r.URL.Query() {
		if key != "name" || (r.Method == http.MethodGet) {
			writeJSON(w, 403, map[string]string{"error": "upload only"})
			return
		}
	}
	s.fileMu.Lock()
	sh, folder, err := s.validateFileRequest(r, initial)
	if err != nil {
		s.fileMu.Unlock()
		fileError(w, err)
		return
	}
	if r.Method == http.MethodGet {
		s.fileMu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = fileRequestPage.Execute(w, struct{ Expires string }{sh.ExpiresAt.UTC().Format(time.RFC3339)})
		return
	}
	name := r.URL.Query().Get("name")
	if !fileRequestName(name) {
		s.fileMu.Unlock()
		writeJSON(w, 400, map[string]string{"error": "invalid filename"})
		return
	}
	if s.fileRequestUploads == nil {
		s.fileRequestUploads = map[string]int{}
	}
	if s.fileRequestUploads[sh.UserID] >= 4 {
		s.fileMu.Unlock()
		w.Header().Set("Retry-After", "5")
		writeJSON(w, 429, map[string]string{"error": "too many uploads; retry later"})
		return
	}
	s.fileRequestUploads[sh.UserID]++
	s.fileMu.Unlock()
	defer func() { s.fileMu.Lock(); s.fileRequestUploads[sh.UserID]--; s.fileMu.Unlock() }()
	temp, n, err := s.stageFile(r.Context(), sh.UserID, http.MaxBytesReader(w, r.Body, maxWebUploadBytes+1), r.ContentLength)
	if err != nil {
		fileError(w, err)
		return
	}
	defer s.removeFileStage(temp)
	// Every upload receives an opaque suffix, independent of existing names.
	receipt := storage.NewID()
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	storedName := stem + "-" + receipt + ext
	rel := cleanRel(filepath.Join(sh.Path, storedName))
	s.fileMu.Lock()
	_, current, err := s.validateFileRequest(r, sh)
	if err == nil && !os.SameFile(folder, current) {
		err = os.ErrPermission
	}
	if err == nil {
		var release func()
		release, err = s.claimFileWrite(sh.UserID, rel, "")
		if err == nil {
			_, err = s.commitFile(sh.UserID, rel, temp, "", fileActor{Source: "file_request"})
			release()
		}
	}
	event := fileActivityEvent{ID: receipt, Path: sh.Path, Action: "file_received", Actor: fileActor{Source: "file_request"}, Subject: storedName}
	if err == nil {
		s.recordFileEvent(sh.UserID, event)
	}
	s.fileMu.Unlock()
	if err != nil {
		fileError(w, err)
		return
	}
	// Notification failure cannot make an already accepted file look rejected.
	notification := fileRequestNotification(receipt, storedName, time.Now().UTC())
	if s.notify != nil {
		for _, recipient := range s.fileOwnerNotificationUsers(r.Context(), sh.UserID) {
			if err := s.notify.Publish(recipient, notification); err != nil {
				s.log.Error("file request notification failed", "owner", sh.UserID, "error", err)
			}
		}
	}
	writeJSON(w, 201, map[string]any{"status": "received", "receipt": receipt, "size": n})
}
func fileRequestNotification(id, name string, at time.Time) notify.Event {
	return notify.Event{ID: id, Kind: notify.KindFiles, Title: "Получен документ по запросу файлов", Body: name, Href: "files", CreatedAt: at}
}

// Caller holds fileMu. Journal-backed notices survive a server restart.
func (s *Server) recentFileRequestNotifications(owner string, limit int) ([]notify.Event, error) {
	state, err := s.fileActivity(owner)
	if err != nil {
		return nil, err
	}
	out := []notify.Event{}
	for i := len(state.Events) - 1; i >= 0 && len(out) < limit; i-- {
		e := state.Events[i]
		if e.Action == "file_received" {
			out = append(out, fileRequestNotification(e.ID, e.Subject, e.At))
		}
	}
	return out, nil
}
