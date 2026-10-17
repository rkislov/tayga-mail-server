package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

type outgoingAttachment struct {
	Name    string `json:"name"`
	Data    string `json:"data"`
	Path    string `json:"path,omitempty"`
	ETag    string `json:"etag,omitempty"`
	content []byte
}

// MIME assembly is bounded independently of the much larger Files upload limit.
const maxMailAttachmentBytes int64 = 10 << 20

func (s *Server) mailLimits(r *http.Request, user *storage.User) map[string]int64 {
	limit := s.cfgLive().SMTP.MaxSize
	if limit <= 0 {
		limit = 25 << 20
	}
	cos := s.userCoS(r, user)
	if cos.MaxMailSize > 0 && cos.MaxMailSize < limit {
		limit = cos.MaxMailSize
	}
	large := cos.LargeAttachBytes
	if large <= 0 || large > maxMailAttachmentBytes {
		large = maxMailAttachmentBytes
	}
	ttl := cos.ShareMaxTTLSec
	if ttl <= 0 || ttl > 7*24*3600 {
		ttl = 7 * 24 * 3600
	}
	return map[string]int64{"message_bytes": limit, "attachment_bytes": maxMailAttachmentBytes, "large_attachment_bytes": large, "share_ttl_hours": max(1, ttl/3600), "upload_bytes": maxWebUploadBytes}
}

func (s *Server) mailAttachmentLimits(w http.ResponseWriter, r *http.Request, au *authUser) {
	user, err := s.store.GetUserByID(r.Context(), au.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "user lookup failed"})
		return
	}
	writeJSON(w, 200, s.mailLimits(r, user))
}

// The client only gets a reference after the upload has been committed. References
// always resolve inside the authenticated user's Files root, never a client path.
func (s *Server) mailAttachmentInfo(w http.ResponseWriter, r *http.Request, au *authUser) {
	var file outgoingAttachment
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&file); err != nil || file.Path == "" || len(file.Name) > 512 {
		writeJSON(w, 400, map[string]string{"error": "invalid attachment reference"})
		return
	}
	root, err := s.filesRoot(au.ID)
	if err != nil {
		fileError(w, err)
		return
	}
	abs, err := safeJoin(root, file.Path)
	if err != nil {
		fileError(w, err)
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		writeJSON(w, 404, map[string]string{"error": "uploaded file not found"})
		return
	}
	writeJSON(w, 200, map[string]any{"name": file.Name, "path": file.Path, "etag": fileETag(info), "size": info.Size()})
}

func (s *Server) resolveOutgoingFiles(userID string, files []outgoingAttachment) ([]outgoingAttachment, error) {
	if len(files) > 20 {
		return nil, fmt.Errorf("too many attachments")
	}
	root, err := s.filesRoot(userID)
	if err != nil {
		return nil, err
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	var total int64
	result := append([]outgoingAttachment(nil), files...)
	for i := range result {
		file := &result[i]
		if file.Path == "" {
			continue
		}
		if file.Data != "" || file.ETag == "" {
			return nil, fmt.Errorf("invalid attachment reference")
		}
		abs, err := safeJoin(root, file.Path)
		if err != nil {
			return nil, err
		}
		f, err := os.Open(abs)
		if err != nil {
			return nil, fmt.Errorf("uploaded attachment not found")
		}
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || fileETag(info) != file.ETag {
			f.Close()
			return nil, fmt.Errorf("uploaded attachment changed; attach it again")
		}
		if info.Size() > maxMailAttachmentBytes-total {
			f.Close()
			return nil, fmt.Errorf("attachments exceed 10 MiB; send large files as links")
		}
		file.content, err = io.ReadAll(io.LimitReader(f, maxMailAttachmentBytes-total+1))
		f.Close()
		if err != nil {
			return nil, err
		}
		total += int64(len(file.content))
		if total > maxMailAttachmentBytes {
			return nil, fmt.Errorf("attachments exceed 10 MiB; send large files as links")
		}
	}
	return result, nil
}

func attachOutgoingFiles(raw []byte, files []outgoingAttachment) ([]byte, error) {
	if len(files) == 0 {
		return raw, nil
	}
	if len(files) > 20 {
		return nil, fmt.Errorf("too many attachments")
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	split := bytes.Index(raw, []byte("Content-Type:"))
	if split < 0 {
		return nil, fmt.Errorf("invalid MIME message")
	}
	part := raw[split:]
	sep := bytes.Index(part, []byte("\r\n\r\n"))
	if sep < 0 {
		return nil, fmt.Errorf("invalid MIME body")
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", strings.TrimSpace(string(part[len("Content-Type:"):sep])))
	writer, err := mw.CreatePart(header)
	if err != nil {
		return nil, err
	}
	writer.Write(part[sep+4:])
	total := 0
	for _, file := range files {
		data := file.content
		var err error
		if file.Path == "" {
			data, err = base64.StdEncoding.DecodeString(file.Data)
		}
		if err != nil {
			return nil, fmt.Errorf("invalid attachment data")
		}
		total += len(data)
		if total > 10<<20 {
			return nil, fmt.Errorf("attachments exceed 10 MiB; upload through Files")
		}
		name := strings.ReplaceAll(strings.ReplaceAll(file.Name, "\r", ""), "\n", "")
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", "application/octet-stream")
		h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
		h.Set("Content-Transfer-Encoding", "base64")
		p, err := mw.CreatePart(h)
		if err != nil {
			return nil, err
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		for len(encoded) > 76 {
			fmt.Fprintf(p, "%s\r\n", encoded[:76])
			encoded = encoded[76:]
		}
		fmt.Fprintf(p, "%s\r\n", encoded)
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	out := append([]byte{}, raw[:split]...)
	out = append(out, []byte("Content-Type: multipart/mixed; boundary="+mw.Boundary()+"\r\n\r\n")...)
	return append(out, body.Bytes()...), nil
}
