package httpapi

import (
	"bytes"
	"embed"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed file_templates/*.docx file_templates/*.xlsx file_templates/*.pptx
var officeTemplates embed.FS

type officeTemplate struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	TitleEN   string `json:"title_en"`
	Extension string `json:"extension"`
}

var officeTemplateCatalog = []officeTemplate{
	{"document", "Пустой документ", "Blank document", ".docx"},
	{"meeting-minutes", "Протокол встречи", "Meeting minutes", ".docx"},
	{"spreadsheet", "Пустая таблица", "Blank spreadsheet", ".xlsx"},
	{"presentation", "Презентация", "Presentation", ".pptx"},
}

const maxOfficeTemplateBytes = 32 << 20

func templateDocumentExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".docx", ".xlsx", ".pptx", ".odt", ".ods", ".odp", ".rtf", ".txt", ".csv":
		return ext
	}
	return ""
}
func (s *Server) handleFileTemplates(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]any{"templates": officeTemplateCatalog, "office_enabled": s.cfgLive().Files.OfficeURL != ""})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		TemplateID    string  `json:"template_id"`
		Directory     string  `json:"directory"`
		Name          string  `json:"name"`
		OwnerID       *string `json:"owner_id"`
		SourcePath    string  `json:"source_path"`
		SourceOwnerID string  `json:"source_owner_id"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	dec.DisallowUnknownFields()
	if dec.Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	if (req.TemplateID == "") == (req.SourcePath == "") {
		writeJSON(w, 400, map[string]string{"error": "choose one template source"})
		return
	}
	owner := au.ID
	if req.OwnerID != nil {
		if *req.OwnerID != "" {
			owner = *req.OwnerID
		}
	} else if v := r.URL.Query().Get("owner_id"); v != "" {
		owner = v
	}
	root, err := s.filesRoot(owner)
	if err != nil {
		fileError(w, err)
		return
	}
	parent, err := safeJoin(root, req.Directory)
	if err != nil {
		fileError(w, err)
		return
	}
	directory := cleanRel(req.Directory)
	allowed := func() bool { return owner == au.ID || s.fileAccessAllowed(r, owner, au.ID, directory, true) }
	if !allowed() {
		writeJSON(w, 403, map[string]string{"error": "destination access denied"})
		return
	}
	sourceOwner := au.ID
	if req.SourceOwnerID != "" {
		sourceOwner = req.SourceOwnerID
	}
	var src io.Reader
	var input *os.File
	var sourceAbs, sourceETag string
	var size int64
	var ext string
	if req.TemplateID != "" {
		for _, item := range officeTemplateCatalog {
			if item.ID == req.TemplateID {
				ext = item.Extension
				break
			}
		}
		if ext == "" {
			writeJSON(w, 404, map[string]string{"error": "template not found"})
			return
		}
		data, e := officeTemplates.ReadFile("file_templates/" + req.TemplateID + ext)
		if e != nil {
			fileError(w, e)
			return
		}
		src, size = bytes.NewReader(data), int64(len(data))
	} else {
		ext = templateDocumentExtension(req.SourcePath)
		if ext == "" {
			writeJSON(w, 400, map[string]string{"error": "unsupported template format"})
			return
		}
		if sourceOwner != au.ID && !s.fileAccessAllowed(r, sourceOwner, au.ID, req.SourcePath, false) {
			writeJSON(w, 403, map[string]string{"error": "template access denied"})
			return
		}
		sourceRoot, e := s.filesRoot(sourceOwner)
		if e != nil {
			fileError(w, e)
			return
		}
		sourceAbs, e = safeJoin(sourceRoot, req.SourcePath)
		if e != nil {
			fileError(w, e)
			return
		}
		input, e = os.Open(sourceAbs)
		if e != nil {
			fileError(w, e)
			return
		}
		defer input.Close()
		info, e := input.Stat()
		if e != nil {
			fileError(w, e)
			return
		}
		if !info.Mode().IsRegular() || info.Size() > maxOfficeTemplateBytes {
			writeJSON(w, 400, map[string]string{"error": "template must be a regular document up to 32 MiB"})
			return
		}
		src, size, sourceETag = input, info.Size(), fileETag(info)
	}
	name := req.Name
	if strings.TrimSpace(name) == "" {
		writeJSON(w, 400, map[string]string{"error": "document name required"})
		return
	}
	if filepath.Ext(name) == "" {
		name += ext
	}
	if !validOfficeTarget(name) || strings.ToLower(filepath.Ext(name)) != ext {
		writeJSON(w, 400, map[string]string{"error": "invalid document name or extension"})
		return
	}
	target := cleanRel(filepath.ToSlash(filepath.Join(directory, name)))
	targetAbs, err := safeJoin(root, target)
	if err != nil {
		fileError(w, err)
		return
	}
	temp, _, err := s.stageFile(r.Context(), owner, src, size)
	if err != nil {
		fileError(w, err)
		return
	}
	defer os.Remove(temp)
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if r.Context().Err() != nil {
		fileError(w, r.Context().Err())
		return
	}
	// Both source and destination permissions are checked again after staging.
	for _, id := range []string{au.ID, owner, sourceOwner} {
		u, e := s.store.GetUserByID(r.Context(), id)
		if e != nil || u == nil || !u.Enabled {
			writeJSON(w, 403, map[string]string{"error": "account unavailable"})
			return
		}
	}
	if !allowed() {
		writeJSON(w, 403, map[string]string{"error": "destination access revoked"})
		return
	}
	if sourceAbs != "" {
		if sourceOwner != au.ID && !s.fileAccessAllowed(r, sourceOwner, au.ID, req.SourcePath, false) {
			writeJSON(w, 403, map[string]string{"error": "template access revoked"})
			return
		}
		current, e := os.Stat(sourceAbs)
		if e != nil || fileETag(current) != sourceETag {
			writeJSON(w, 409, map[string]string{"error": "template changed; reload and retry"})
			return
		}
	}
	// Re-resolve after staging so a changed parent cannot introduce a symlink.
	if _, err = safeJoin(root, target); err != nil {
		fileError(w, err)
		return
	}
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		writeJSON(w, 404, map[string]string{"error": "destination folder not found"})
		return
	}
	if _, err = os.Lstat(targetAbs); err == nil {
		writeJSON(w, 409, map[string]string{"error": "destination already exists"})
		return
	} else if !os.IsNotExist(err) {
		fileError(w, err)
		return
	}
	release, err := s.claimFileWrite(owner, target, "")
	if err != nil {
		fileError(w, err)
		return
	}
	defer release()
	etag, err := s.commitFile(owner, target, temp, "")
	if err != nil {
		fileError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"path": target, "name": name, "owner_id": owner, "size": size, "etag": etag, "office_enabled": s.cfgLive().Files.OfficeURL != ""})
}
