package httpapi

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

// Prepare an owned attachment in Files without a browser download/upload round
// trip. It then uses the same quota, immutable reference and large-file link
// flow as a newly uploaded attachment. This endpoint never sends a message.
func (s *Server) mailPrepareForwardAttachment(w http.ResponseWriter, r *http.Request, au *authUser, messageID, attachmentID string) {
	msg, _, code := s.messageOwned(r, au, messageID)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": http.StatusText(code)})
		return
	}
	raw, err := s.ms.Read(msg.FilePath)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "cannot read source message"})
		return
	}
	for _, a := range parseMIMEMessage(raw).Attachments {
		if a.ID != attachmentID {
			continue
		}
		// The MIME parser bounds each part at 64 MiB. Never forward a possibly
		// truncated part as if it were the complete attachment.
		if len(a.Data) >= 64<<20 {
			writeJSON(w, 413, map[string]string{"error": "attachment exceeds forwarding parser limit (64 MiB)"})
			return
		}
		name := strings.Map(func(r rune) rune {
			if r < 32 || r == 127 || r == '/' || r == '\\' {
				return '_'
			}
			return r
		}, a.Filename)
		for len(name) > 180 {
			name = string([]rune(name)[:len([]rune(name))-1])
		}
		if name == "" || name == "." || name == ".." {
			name = "attachment"
		}
		rel := "mail-out/" + storage.NewID() + "/" + name
		copyRequest := r.Clone(r.Context())
		copyRequest.Body = io.NopCloser(bytes.NewReader(a.Data))
		copyRequest.ContentLength = int64(len(a.Data))
		copyRequest.Header = make(http.Header)
		copyRequest.Header.Set("If-None-Match", "*")
		size, err := s.saveFile(w, copyRequest, au.ID, rel)
		if err != nil {
			fileError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"name": a.Filename, "path": rel, "etag": w.Header().Get("ETag"), "size": size})
		return
	}
	writeJSON(w, 404, map[string]string{"error": "attachment not found"})
}
