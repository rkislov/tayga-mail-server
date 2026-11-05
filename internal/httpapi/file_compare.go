package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type fileCompareSession struct {
	Owner, Path, Left, Right, LeftETag, RightETag, Provider, OfficeURL, Language, Mode string
	SecretHash                                                                         [32]byte
	Expires                                                                            time.Time
}

// Caller holds fileMu. Both sides must belong to this owner's same document.
func (s *Server) compareFile(owner, path, id string) (*os.File, string, error) {
	var abs string
	if id == "current" {
		root, err := s.filesRoot(owner)
		if err != nil {
			return nil, "", err
		}
		abs, err = safeJoin(root, path)
		if err != nil {
			return nil, "", err
		}
	} else {
		rev, dir, err := s.revision(owner, "versions", id)
		if err != nil {
			return nil, "", err
		}
		if rev.IsDir || rev.Path != path {
			return nil, "", os.ErrNotExist
		}
		abs, err = safeJoin(dir, "content")
		if err != nil {
			return nil, "", err
		}
	}
	file, err := os.Open(abs)
	if err != nil {
		return nil, "", err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, "", err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, "", os.ErrNotExist
	}
	if info.Size() > maxCompareFileBytes {
		file.Close()
		return nil, "", errFileTooLarge
	}
	return file, fileETag(info), nil
}
func (s *Server) handleFileComparison(w http.ResponseWriter, r *http.Request, owner string) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Path     string `json:"path"`
		Left     string `json:"left"`
		Right    string `json:"right"`
		Mode     string `json:"mode"`
		Language string `json:"lang"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || req.Path == "" || req.Left == "" {
		writeJSON(w, 400, map[string]string{"error": "path and left revision required"})
		return
	}
	if req.Right == "" {
		req.Right = "current"
	}
	if req.Mode == "" {
		req.Mode = "text"
		if structuredCompareFormat(req.Path) {
			req.Mode = "structured"
		}
	}
	path := cleanRel(req.Path)
	if req.Left == req.Right {
		writeJSON(w, 400, map[string]string{"error": "choose two different versions"})
		return
	}
	if (req.Mode == "text" && !textCompareFormat(path)) || (req.Mode == "office" && !officeCompareFormat(path)) || ((req.Mode == "structured" || req.Mode == "details") && !structuredCompareFormat(path)) || (req.Mode == "visual" && !visualCompareFormat(path)) || (req.Mode == "motion" && !motionCompareFormat(path)) || (req.Mode != "text" && req.Mode != "office" && req.Mode != "structured" && req.Mode != "details" && req.Mode != "visual" && req.Mode != "motion") {
		writeJSON(w, 415, map[string]string{"error": "comparison format unsupported"})
		return
	}
	// Even a historical-only request must contain a valid managed path.
	root, err := s.filesRoot(owner)
	if err != nil {
		fileComparisonError(w, err)
		return
	}
	if _, err = safeJoin(root, req.Path); err != nil {
		fileComparisonError(w, err)
		return
	}
	s.fileMu.Lock()
	left, leftTag, err := s.compareFile(owner, path, req.Left)
	if err != nil {
		s.fileMu.Unlock()
		fileComparisonError(w, err)
		return
	}
	defer left.Close()
	right, rightTag, err := s.compareFile(owner, path, req.Right)
	if err != nil {
		s.fileMu.Unlock()
		fileComparisonError(w, err)
		return
	}
	defer right.Close()
	if req.Mode == "structured" || req.Mode == "details" {
		s.fileMu.Unlock()
		a, e := compareStructuredDocument(left, path, req.Mode == "details")
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": e.Error()})
			return
		}
		b, e := compareStructuredDocument(right, path, req.Mode == "details")
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": e.Error()})
			return
		}
		if r.Context().Err() != nil {
			return
		}
		changes, added, removed, modified := structuredDocumentDiff(a, b)
		writeJSON(w, 200, map[string]any{"mode": "structured", "details": a.details, "scope": a.scope, "path": path, "left": req.Left, "right": req.Right, "left_etag": leftTag, "right_etag": rightTag, "added": added, "removed": removed, "modified": modified, "changes": changes})
		return
	}
	if req.Mode == "text" {
		// File descriptors remain stable across atomic replacements/deletion.
		s.fileMu.Unlock()
		a, e := compareDocumentText(left, path)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": e.Error()})
			return
		}
		b, e := compareDocumentText(right, path)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": e.Error()})
			return
		}
		if r.Context().Err() != nil {
			return
		}
		lines, added, removed := documentLineDiff(a, b)
		writeJSON(w, 200, map[string]any{"mode": "text", "path": path, "left": req.Left, "right": req.Right, "left_etag": leftTag, "right_etag": rightTag, "added": added, "removed": removed, "lines": lines, "scope": "document-text"})
		return
	}
	defer s.fileMu.Unlock()
	cfg := s.cfgLive().Files
	base, e := url.Parse(strings.TrimRight(cfg.OfficeURL, "/"))
	public, pe := url.Parse(s.publicBase())
	validOffice := e == nil && base.Scheme == "https" && base.Host != "" && base.User == nil && base.RawQuery == "" && base.Fragment == ""
	documentServer := (cfg.OfficeProvider == "onlyoffice" || cfg.OfficeProvider == "r7") && len(cfg.OfficeJWTSecret) >= 32 && pe == nil && public.Scheme == "https" && public.Host != ""
	collabora := (req.Mode == "visual" || req.Mode == "motion") && (cfg.OfficeProvider == "collabora" || cfg.OfficeProvider == "")
	plainPDF := req.Mode == "visual" && strings.EqualFold(filepath.Ext(path), ".pdf")
	if !plainPDF && (!validOffice || (!documentServer && !collabora)) {
		writeJSON(w, 503, map[string]string{"error": "comparison requires a configured compatible office conversion service"})
		return
	}
	count := 0
	for key, value := range s.fileComparisons {
		if time.Now().After(value.Expires) {
			delete(s.fileComparisons, key)
		} else if value.Owner == owner {
			count++
		}
	}
	if count >= 32 || len(s.fileComparisons) >= 1024 {
		writeJSON(w, 429, map[string]string{"error": "too many open comparisons"})
		return
	}
	var entropy [32]byte
	if _, err = rand.Read(entropy[:]); err != nil {
		fileComparisonError(w, err)
		return
	}
	token := hex.EncodeToString(entropy[:])
	if s.fileComparisons == nil {
		s.fileComparisons = map[string]fileCompareSession{}
	}
	s.fileComparisons[token] = fileCompareSession{Mode: req.Mode, Owner: owner, Path: path, Left: req.Left, Right: req.Right, LeftETag: leftTag, RightETag: rightTag, Provider: cfg.OfficeProvider, OfficeURL: strings.TrimRight(cfg.OfficeURL, "/"), Language: req.Language, SecretHash: sha256.Sum256([]byte(cfg.OfficeJWTSecret)), Expires: time.Now().Add(30 * time.Minute)}
	if req.Mode == "motion" {
		endpoint := "/office/compare/"
		query := "?access_token=" + token
		writeJSON(w, 200, map[string]string{"mode": "motion", "left_timeline": endpoint + "timeline" + query + "&side=left", "right_timeline": endpoint + "timeline" + query + "&side=right", "left_url": endpoint + "pdf" + query + "&side=left&storyboard=1", "right_url": endpoint + "pdf" + query + "&side=right&storyboard=1"})
		return
	}
	if req.Mode == "visual" {
		endpoint := "/office/compare/pdf?access_token=" + token
		writeJSON(w, 200, map[string]string{"mode": "visual", "left_url": endpoint + "&side=left", "right_url": endpoint + "&side=right"})
		return
	}
	writeJSON(w, 200, map[string]string{"mode": "office", "url": s.publicBase() + "/office/compare/editor?access_token=" + token})
}

var fileComparePage = template.Must(template.New("compare-editor").Parse(`<!doctype html><html><head><meta charset="utf-8"><meta name="referrer" content="no-referrer"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Compare versions</title><style>html,body,#editor{margin:0;width:100%;height:100%;overflow:hidden}#compare-error{position:absolute;top:0;background:white;color:#b91c1c;padding:1rem;z-index:10}</style></head><body><div id="editor"></div><p id="compare-error" role="alert" hidden></p><script src="{{.Script}}"></script><script>const config={{.Config}};const revised={{.Revised}};const showError=message=>{const node=document.getElementById('compare-error');node.textContent=message;node.hidden=false;};config.events={onDocumentReady:function(){if(typeof editor.setRequestedDocument!=='function'){showError('Сравнение недоступно в этой версии редактора / Comparison is unavailable in this editor version');return;}editor.setRequestedDocument(revised);},onError:function(event){showError(event.data.errorDescription);}};let editor;if(typeof DocsAPI==='undefined'){showError('Не удалось загрузить редактор / Could not load editor');}else{editor=new DocsAPI.DocEditor('editor',config);addEventListener('pagehide',()=>editor.destroyEditor());}</script></body></html>`))

func (s *Server) handleOfficeComparison(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	action := strings.TrimPrefix(r.URL.Path, "/office/compare/")
	if action != "editor" && action != "content" && action != "pdf" && action != "timeline" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && !(action == "content" && r.Method == http.MethodHead) {
		w.WriteHeader(405)
		return
	}
	token := r.URL.Query().Get("access_token")
	s.fileMu.Lock()
	sess, ok := s.fileComparisons[token]
	cfg := s.cfgLive().Files
	if !ok || len(token) != 64 || time.Now().After(sess.Expires) || sess.Provider != cfg.OfficeProvider || sess.OfficeURL != strings.TrimRight(cfg.OfficeURL, "/") || sess.SecretHash != sha256.Sum256([]byte(cfg.OfficeJWTSecret)) {
		s.fileMu.Unlock()
		http.Error(w, "unauthorized", 401)
		return
	}
	if (action == "pdf" && sess.Mode != "visual" && sess.Mode != "motion") || (action == "timeline" && sess.Mode != "motion") || (action == "editor" && (sess.Mode == "visual" || sess.Mode == "motion")) || ((r.URL.Query().Get("storyboard") != "") != (sess.Mode == "motion" && action != "timeline")) {
		s.fileMu.Unlock()
		http.Error(w, "invalid comparison mode", 403)
		return
	}
	user, err := s.store.GetUserByID(r.Context(), sess.Owner)
	if err != nil || user == nil || !user.Enabled {
		s.fileMu.Unlock()
		http.Error(w, "unauthorized", 401)
		return
	}
	a, aTag, err := s.compareFile(sess.Owner, sess.Path, sess.Left)
	if err != nil {
		s.fileMu.Unlock()
		fileComparisonError(w, err)
		return
	}
	defer a.Close()
	b, bTag, err := s.compareFile(sess.Owner, sess.Path, sess.Right)
	if err != nil {
		s.fileMu.Unlock()
		fileComparisonError(w, err)
		return
	}
	defer b.Close()
	if aTag != sess.LeftETag || bTag != sess.RightETag {
		s.fileMu.Unlock()
		http.Error(w, "comparison version changed; reopen", 409)
		return
	}
	s.fileMu.Unlock()
	if action == "content" || action == "pdf" || action == "timeline" {
		file := a
		switch r.URL.Query().Get("side") {
		case "left":
		case "right":
			file = b
		default:
			http.Error(w, "invalid side", 400)
			return
		}
		if sess.Mode == "motion" {
			deck, temporary, cleanup, err := prepareMotionSource(file, sess.Path, action != "timeline")
			if err != nil {
				status := 422
				if errors.Is(err, errMotionBusy) {
					status = 429
				}
				writeJSON(w, status, map[string]string{"error": err.Error()})
				return
			}
			defer cleanup()
			if status := s.comparisonAfterWork(r.Context(), sess, token); status != 200 {
				http.Error(w, "comparison changed; reopen", status)
				return
			}
			if action == "timeline" {
				writeJSON(w, 200, deck)
				return
			}
			file = temporary
		}
		if action == "pdf" {
			s.serveComparisonPDF(w, r, sess, token, file)
			return
		}
		info, e := file.Stat()
		if e != nil {
			fileError(w, e)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, filepath.Base(sess.Path), info.ModTime(), file)
		return
	}
	endpoint := s.publicBase() + "/office/compare/content?access_token=" + token
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(sess.Path)), ".")
	generation := sha256.Sum256([]byte("tayga-compare\x00" + token))
	config := map[string]any{"documentType": "word", "width": "100%", "height": "100%", "document": map[string]any{"fileType": ext, "title": filepath.Base(sess.Path), "key": hex.EncodeToString(generation[:]), "url": endpoint + "&side=left", "permissions": map[string]bool{"edit": false, "review": false, "comment": false, "download": false, "print": false, "copy": false}}, "editorConfig": map[string]any{"mode": "view", "lang": sess.Language, "user": map[string]string{"id": sess.Owner, "name": user.DisplayName}}}
	revised := map[string]any{"c": "compare", "fileType": ext, "url": endpoint + "&side=right"}
	signed, err := signDocumentConfig(config, cfg.OfficeJWTSecret)
	if err != nil {
		fileComparisonError(w, err)
		return
	}
	config["token"] = signed
	signed, err = signDocumentConfig(revised, cfg.OfficeJWTSecret)
	if err != nil {
		fileComparisonError(w, err)
		return
	}
	revised["token"] = signed
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'; object-src 'none'; base-uri 'none'")
	_ = fileComparePage.Execute(w, map[string]any{"Script": sess.OfficeURL + "/web-apps/apps/api/documents/api.js", "Config": config, "Revised": revised})
}

func fileComparisonError(w http.ResponseWriter, err error) {
	if errors.Is(err, errFileTooLarge) {
		http.Error(w, "document exceeds 32 MiB comparison limit", 413)
		return
	}
	fileError(w, err)
}
