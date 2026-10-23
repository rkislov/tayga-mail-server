package httpapi

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleCalendarDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	token := strings.TrimPrefix(r.URL.Path, storage.CalendarDownloadPath)
	a, err := s.store.CalendarDownloadAttachment(r.Context(), token)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.serveCalendarFile(w, r, a)
}

func (s *Server) serveCalendarFile(w http.ResponseWriter, r *http.Request, a *storage.CalendarAttachment) {
	if s.ms == nil || !filepath.IsLocal(a.StoragePath) || a.Size < 0 || a.Size > maxCalAttachmentBytes {
		http.NotFound(w, r)
		return
	}
	root, err := os.OpenRoot(filepath.Join(s.ms.Root, "calendar-attachments"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer root.Close()
	file, err := root.Open(filepath.FromSlash(a.StoragePath))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != a.Size {
		http.NotFound(w, r)
		return
	}
	ct := a.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": a.Filename}))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, a.Filename, info.ModTime(), file)
}
