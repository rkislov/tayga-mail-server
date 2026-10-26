package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/net/webdav"
)

func documentServerType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".docx", ".doc", ".odt", ".rtf", ".txt":
		return "word"
	case ".xlsx", ".xls", ".ods", ".csv":
		return "cell"
	case ".pptx", ".ppt", ".odp":
		return "slide"
	}
	return ""
}

func signDocumentConfig(value map[string]any, secret string) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(value)).SignedString([]byte(secret))
}

func (s *Server) createDocumentServerSession(w http.ResponseWriter, r *http.Request, owner, user, path, base string, writable bool) {
	cfg := s.cfgLive().Files
	if len(cfg.OfficeJWTSecret) < 32 {
		writeJSON(w, 503, map[string]string{"error": "Set files.office_jwt_secret (at least 32 bytes) for ONLYOFFICE/R7."})
		return
	}
	if documentServerType(path) == "" {
		writeJSON(w, 400, map[string]string{"error": "Unsupported document format"})
		return
	}
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		fileError(w, err)
		return
	}
	token := hex.EncodeToString(entropy[:])
	// The collaboration key is visible to every participant. Derive it one-way
	// so it can never be used as the first participant's scoped access token.
	generation := sha256.Sum256([]byte("document-server-generation\x00" + token))
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	count := 0
	now := time.Now()
	for key, sess := range s.officeSessions {
		if now.After(sess.Expires) {
			delete(s.officeSessions, key)
		} else if sess.User == user && !sess.Closed {
			count++
		}
	}
	if count >= 64 || len(s.officeSessions) >= 4096 {
		writeJSON(w, 429, map[string]string{"error": "too many open documents"})
		return
	}
	root, err := s.filesRoot(owner)
	if err != nil {
		fileError(w, err)
		return
	}
	abs, err := safeJoin(root, path)
	if err != nil {
		fileError(w, err)
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "not found", 404)
		return
	}
	if owner != user && !s.fileAccessAllowed(r, owner, user, path, writable) {
		http.Error(w, "file access revoked", 403)
		return
	}
	sess := officeSession{Owner: owner, User: user, Path: cleanRel(path), FileID: officeFileID(owner, path), OfficeURL: base, Provider: cfg.OfficeProvider, Writable: writable, Expires: now.Add(8 * time.Hour), ETag: fileETag(info), DocumentKey: hex.EncodeToString(generation[:]), Language: r.URL.Query().Get("lang"), SecretHash: sha256.Sum256([]byte(cfg.OfficeJWTSecret))}
	// All participants share a generation, including after force-save. A final
	// callback closes it; reopening receives a new key and bypasses editor caches.
	for _, prior := range s.officeSessions {
		if prior.Owner == owner && prior.Path == sess.Path && prior.Provider == sess.Provider && prior.OfficeURL == base && prior.SecretHash == sess.SecretHash && !prior.Closed && now.Before(prior.Expires) && prior.ETag == sess.ETag {
			sess.DocumentKey = prior.DocumentKey
			break
		}
	}
	if writable {
		ls := s.locksFor(owner)
		key := "/" + sess.Path
		lock := ls.office[key]
		// A settings change revokes callbacks. Release a native lock whose
		// generation belongs entirely to the old configuration so reopening
		// does not wait eight hours for an editor that can no longer save.
		revoked, live := false, false
		for _, prior := range s.officeSessions {
			if prior.Owner == owner && prior.Path == sess.Path && prior.DocumentKey == lock.Value && (prior.Provider == "onlyoffice" || prior.Provider == "r7") {
				revoked = true
				if !prior.Closed && now.Before(prior.Expires) && prior.Provider == sess.Provider && prior.OfficeURL == base && prior.SecretHash == sess.SecretHash {
					live = true
				}
			}
		}
		if revoked && !live {
			_ = ls.Unlock(now, lock.Token)
			delete(ls.office, key)
			lock = officeFileLock{}
		}
		if now.Before(lock.Expires) && lock.Value == sess.DocumentKey {
			if _, err = ls.Refresh(now, lock.Token, 8*time.Hour); err != nil {
				fileError(w, errFileLocked)
				return
			}
			lock.Expires = sess.Expires
			ls.office[key] = lock
		} else {
			lockToken, e := ls.Create(now, webdav.LockDetails{Root: key, Duration: 8 * time.Hour, ZeroDepth: true})
			if e != nil {
				fileError(w, errFileLocked)
				return
			}
			ls.office[key] = officeFileLock{Value: sess.DocumentKey, Token: lockToken, Expires: sess.Expires}
		}
	}
	if s.officeSessions == nil {
		s.officeSessions = map[string]officeSession{}
	}
	s.officeSessions[token] = sess
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"provider": cfg.OfficeProvider, "url": s.publicBase() + "/office/document/editor?access_token=" + token, "writable": writable, "file_id": sess.FileID})
}

// Caller holds fileMu. Every request rechecks live configuration, accounts and ACL.
func (s *Server) documentSession(r *http.Request, token string, write bool) (officeSession, bool) {
	sess, ok := s.officeSessions[token]
	cfg := s.cfgLive().Files
	if !ok || len(token) != 64 || (sess.Provider != "onlyoffice" && sess.Provider != "r7") || sess.Provider != cfg.OfficeProvider || sess.OfficeURL != strings.TrimRight(cfg.OfficeURL, "/") || sess.SecretHash != sha256.Sum256([]byte(cfg.OfficeJWTSecret)) || time.Now().After(sess.Expires) {
		return sess, false
	}
	user, e := s.store.GetUserByID(r.Context(), sess.User)
	owner, oe := s.store.GetUserByID(r.Context(), sess.Owner)
	if e != nil || oe != nil || user == nil || owner == nil || !user.Enabled || !owner.Enabled || (write && !sess.Writable) {
		return sess, false
	}
	if sess.Owner != sess.User && !s.fileAccessAllowed(r, sess.Owner, sess.User, sess.Path, write) {
		return sess, false
	}
	return sess, true
}

var documentEditorPage = template.Must(template.New("document-editor").Parse(`<!doctype html><html><head><meta charset="utf-8"><meta name="referrer" content="no-referrer"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Office</title><style>html,body,#editor{margin:0;width:100%;height:100%;overflow:hidden}</style></head><body><div id="editor"></div><p id="office-error" role="alert" hidden style="position:absolute;top:0;left:0;right:0;padding:1rem;background:white;color:#b91c1c;z-index:1000"></p><script src="{{.Script}}"></script><script>const config={{.Config}};config.events={onError:function(event){const message=document.getElementById('office-error');message.textContent=event.data.errorDescription;message.hidden=false;}};if(typeof DocsAPI==='undefined'){document.getElementById('editor').textContent={{.LoadError}};}else{const editor=new DocsAPI.DocEditor('editor',config);addEventListener('pagehide',()=>editor.destroyEditor());}</script></body></html>`))

func (s *Server) handleDocumentServer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	action := strings.TrimPrefix(r.URL.Path, "/office/document/")
	if action != "editor" && action != "content" && action != "callback" {
		http.NotFound(w, r)
		return
	}
	method := http.MethodGet
	if action == "callback" {
		method = http.MethodPost
	}
	if r.Method != method {
		w.WriteHeader(405)
		return
	}
	token := r.URL.Query().Get("access_token")
	s.fileMu.Lock()
	sess, ok := s.documentSession(r, token, false)
	if !ok {
		s.fileMu.Unlock()
		http.Error(w, "unauthorized", 401)
		return
	}
	if action == "callback" {
		s.fileMu.Unlock()
		s.documentServerCallback(w, r, token, sess)
		return
	}
	if sess.Closed {
		s.fileMu.Unlock()
		http.Error(w, "document session closed", 410)
		return
	}
	root, err := s.filesRoot(sess.Owner)
	if err != nil {
		s.fileMu.Unlock()
		fileError(w, err)
		return
	}
	abs, err := safeJoin(root, sess.Path)
	if err != nil {
		s.fileMu.Unlock()
		fileError(w, err)
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		s.fileMu.Unlock()
		http.NotFound(w, r)
		return
	}
	if fileETag(info) != sess.ETag {
		s.fileMu.Unlock()
		http.Error(w, "document changed", 409)
		return
	}
	if action == "content" {
		f, e := os.Open(abs)
		s.fileMu.Unlock()
		if e != nil {
			fileError(w, e)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
		return
	}
	writable := sess.Writable && (sess.Owner == sess.User || s.fileAccessAllowed(r, sess.Owner, sess.User, sess.Path, true))
	s.fileMu.Unlock()
	mode := "view"
	if writable {
		mode = "edit"
	}
	displayName := sess.User
	if user, e := s.store.GetUserByID(r.Context(), sess.User); e == nil && user != nil && user.DisplayName != "" {
		displayName = user.DisplayName
	}
	endpoint := s.publicBase() + "/office/document/"
	config := map[string]any{
		"documentType": documentServerType(sess.Path), "width": "100%", "height": "100%",
		"document":     map[string]any{"title": info.Name(), "fileType": strings.TrimPrefix(strings.ToLower(filepath.Ext(sess.Path)), "."), "key": sess.DocumentKey, "url": endpoint + "content?access_token=" + token, "permissions": map[string]any{"edit": writable, "review": writable, "comment": writable, "fillForms": writable}},
		"editorConfig": map[string]any{"mode": mode, "lang": sess.Language, "callbackUrl": endpoint + "callback?access_token=" + token, "user": map[string]string{"id": sess.User, "name": displayName}, "customization": map[string]any{"forcesave": true}},
	}
	signed, err := signDocumentConfig(config, s.cfgLive().Files.OfficeJWTSecret)
	if err != nil {
		fileError(w, err)
		return
	}
	config["token"] = signed
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'; object-src 'none'; base-uri 'none'")
	_ = documentEditorPage.Execute(w, map[string]any{"Script": sess.OfficeURL + "/web-apps/apps/api/documents/api.js", "Config": config, "LoadError": "Не удалось загрузить редактор / Could not load the editor"})
}

type documentCallback struct {
	Key      string `json:"key"`
	Status   int    `json:"status"`
	URL      string `json:"url"`
	FileType string `json:"filetype"`
}

func (s *Server) documentServerCallback(w http.ResponseWriter, r *http.Request, token string, sess officeSession) {
	fail := func(status int) { writeJSON(w, status, map[string]int{"error": 1}) }
	var body struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body) != nil {
		fail(400)
		return
	}
	signed := body.Token
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		signed = strings.TrimPrefix(auth, "Bearer ")
	}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(signed, claims, func(*jwt.Token) (any, error) { return []byte(s.cfgLive().Files.OfficeJWTSecret), nil }, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !parsed.Valid {
		fail(401)
		return
	}
	// Outbox header tokens wrap callback data in payload; body tokens use the
	// callback fields directly. Never trust unsigned URL/status fields in the body.
	var payload any = claims
	if value, exists := claims["payload"]; exists {
		payload = value
	}
	raw, err := json.Marshal(payload)
	var cb documentCallback
	if err != nil || json.Unmarshal(raw, &cb) != nil || cb.Key != sess.DocumentKey {
		fail(400)
		return
	}
	if cb.Status != 1 && cb.Status != 2 && cb.Status != 3 && cb.Status != 4 && cb.Status != 6 && cb.Status != 7 {
		fail(400)
		return
	}
	if cb.Status == 3 || cb.Status == 7 {
		fail(200)
		return
	}
	s.fileMu.Lock()
	current, ok := s.documentSession(r, token, cb.Status == 2 || cb.Status == 6)
	if !ok {
		s.fileMu.Unlock()
		fail(403)
		return
	}
	if current.Closed {
		s.fileMu.Unlock()
		writeJSON(w, 200, map[string]int{"error": 0})
		return
	}
	s.fileMu.Unlock()
	if cb.Status == 2 || cb.Status == 6 {
		download, e := url.Parse(cb.URL)
		configured, _ := url.Parse(sess.OfficeURL)
		// The callback is authenticated, but the URL must still stay on the
		// configured document server; redirects are disabled by officeHTTPClient.
		if e != nil || download.Scheme != "https" || !strings.EqualFold(download.Host, configured.Host) || download.User != nil || download.Fragment != "" {
			fail(400)
			return
		}
		extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(sess.Path)), ".")
		if cb.FileType != "" && strings.ToLower(cb.FileType) != extension {
			fail(409)
			return
		}
		req, e := http.NewRequestWithContext(r.Context(), http.MethodGet, download.String(), nil)
		if e != nil {
			fail(400)
			return
		}
		res, e := officeHTTPClient.Do(req)
		if e != nil {
			fail(502)
			return
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			fail(502)
			return
		}
		temp, _, e := s.stageFile(r.Context(), sess.Owner, res.Body, res.ContentLength)
		if e != nil {
			fail(500)
			return
		}
		defer os.Remove(temp)
		s.fileMu.Lock()
		defer s.fileMu.Unlock()
		current, ok = s.documentSession(r, token, true)
		if !ok {
			fail(403)
			return
		}
		if current.Closed {
			writeJSON(w, 200, map[string]int{"error": 0})
			return
		}
		// Compare the ETag captured before downloading: concurrent callbacks must
		// never overwrite a newer save with an older downloaded snapshot.
		if current.ETag != sess.ETag {
			fail(409)
			return
		}
		release, e := s.claimFileWrite(sess.Owner, sess.Path, sess.DocumentKey)
		if e != nil {
			fail(409)
			return
		}
		etag, e := s.commitFile(sess.Owner, sess.Path, temp, current.ETag)
		release()
		if e != nil {
			fail(409)
			return
		}
		for key, participant := range s.officeSessions {
			if participant.DocumentKey == sess.DocumentKey {
				participant.ETag = etag
				s.officeSessions[key] = participant
			}
		}
		if cb.Status == 2 {
			s.closeDocumentGeneration(sess)
		}
	} else if cb.Status == 4 {
		s.fileMu.Lock()
		defer s.fileMu.Unlock()
		s.closeDocumentGeneration(sess)
	}
	writeJSON(w, 200, map[string]int{"error": 0})
}

// Caller holds fileMu. Closed sessions remain briefly idempotent for callbacks;
// their content and editor endpoints can no longer be used.
func (s *Server) closeDocumentGeneration(sess officeSession) {
	ls := s.locksFor(sess.Owner)
	key := "/" + sess.Path
	if lock := ls.office[key]; lock.Value == sess.DocumentKey {
		_ = ls.Unlock(time.Now(), lock.Token)
		delete(ls.office, key)
	}
	for token, participant := range s.officeSessions {
		if participant.DocumentKey == sess.DocumentKey {
			participant.Closed = true
			participant.Expires = time.Now().Add(15 * time.Minute)
			s.officeSessions[token] = participant
		}
	}
}
