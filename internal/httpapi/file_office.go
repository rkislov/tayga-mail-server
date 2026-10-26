package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

// Tokens are document/user-scoped, never login tokens. They expire after 8h
// and intentionally cease working on restart; no browser credentials reach CODE.
type officeSession struct {
	Owner, User, Path, FileID, OfficeURL  string
	Writable                              bool
	Expires                               time.Time
	Proof                                 officeProofKeys
	Provider, DocumentKey, ETag, Language string
	SecretHash                            [32]byte
	Closed                                bool
}

var officePlaceholder = regexp.MustCompile(`<[^>]*>`)
var officeHTTPClient = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func officeFileID(owner, path string) string {
	sum := sha256.Sum256([]byte(owner + "\x00" + cleanRel(path)))
	return hex.EncodeToString(sum[:])
}

func (s *Server) handleOfficeSession(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	base := strings.TrimRight(s.cfgLive().Files.OfficeURL, "/")
	if r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]any{"enabled": base != "", "webdav_url": s.publicBase() + "/dav/files/"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if base == "" {
		writeJSON(w, 503, map[string]string{"error": "Office editor is not configured. Set files.office_url in server settings."})
		return
	}
	public, err := url.Parse(s.publicBase())
	if err != nil || public.Scheme != "https" || public.Host == "" {
		writeJSON(w, 503, map[string]string{"error": "Set http.public_url to the public HTTPS URL before enabling office editing."})
		return
	}
	var req struct {
		Path  string `json:"path"`
		Owner string `json:"owner_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	owner := au.ID
	if req.Owner != "" {
		owner = req.Owner
	} else if r.URL.Query().Get("owner_id") != "" {
		owner = r.URL.Query().Get("owner_id")
	}
	writable := true
	if owner != au.ID {
		if !s.fileAccessAllowed(r, owner, au.ID, req.Path, false) {
			writeJSON(w, 403, map[string]string{"error": "file access denied"})
			return
		}
		writable = s.fileAccessAllowed(r, owner, au.ID, req.Path, true)
	}
	root, err := s.filesRoot(owner)
	if err != nil {
		fileError(w, err)
		return
	}
	abs, err := safeJoin(root, req.Path)
	if err != nil {
		fileError(w, err)
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		writeJSON(w, 404, map[string]string{"error": "file not found"})
		return
	}
	provider := s.cfgLive().Files.OfficeProvider
	if provider == "onlyoffice" || provider == "r7" {
		s.createDocumentServerSession(w, r, owner, au.ID, req.Path, base, writable)
		return
	}
	launch, proof, err := officeLaunchURL(r, base, strings.TrimPrefix(strings.ToLower(filepath.Ext(abs)), "."), writable)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	var entropy [32]byte
	if _, err = rand.Read(entropy[:]); err != nil {
		fileError(w, err)
		return
	}
	token := hex.EncodeToString(entropy[:])
	expires := time.Now().Add(8 * time.Hour)
	sess := officeSession{Owner: owner, User: au.ID, Path: cleanRel(req.Path), FileID: officeFileID(owner, req.Path), OfficeURL: base, Writable: writable, Expires: expires}
	sess.Proof = proof
	s.fileMu.Lock()
	if s.officeSessions == nil {
		s.officeSessions = map[string]officeSession{}
	}
	count := 0
	for key, value := range s.officeSessions {
		if time.Now().After(value.Expires) {
			delete(s.officeSessions, key)
		} else if value.User == au.ID {
			count++
		}
	}
	if count >= 64 || len(s.officeSessions) >= 4096 {
		s.fileMu.Unlock()
		writeJSON(w, 429, map[string]string{"error": "too many open documents"})
		return
	}
	s.officeSessions[token] = sess
	s.fileMu.Unlock()
	query := launch.Query()
	query.Set("WOPISrc", s.publicBase()+"/wopi/files/"+sess.FileID)
	query.Set("lang", r.URL.Query().Get("lang"))
	launch.RawQuery = query.Encode()
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"url": launch.String(), "access_token": token, "access_token_ttl": expires.UnixMilli(), "file_id": sess.FileID, "writable": writable})
}

func officeLaunchURL(r *http.Request, base, extension string, writable bool) (*url.URL, officeProofKeys, error) {
	var proof officeProofKeys
	configured, err := url.Parse(base)
	if err != nil || configured.Scheme != "https" || configured.Host == "" || configured.User != nil {
		return nil, proof, fmt.Errorf("invalid office URL")
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, base+"/hosting/discovery", nil)
	if err != nil {
		return nil, proof, err
	}
	res, err := officeHTTPClient.Do(req)
	if err != nil {
		return nil, proof, fmt.Errorf("office discovery unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, proof, fmt.Errorf("office discovery returned HTTP %d", res.StatusCode)
	}
	var doc struct {
		Proof officeProofKeys `xml:"proof-key"`
		Zones []struct {
			Apps []struct {
				Actions []struct {
					Name string `xml:"name,attr"`
					Ext  string `xml:"ext,attr"`
					URL  string `xml:"urlsrc,attr"`
				} `xml:"action"`
			} `xml:"app"`
		} `xml:"net-zone"`
	}
	if xml.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&doc) != nil {
		return nil, proof, fmt.Errorf("invalid office discovery")
	}
	proof = doc.Proof
	if proof.Modulus != "" && proof.key(false) == nil {
		return nil, proof, fmt.Errorf("invalid office proof key")
	}
	want := "view"
	if writable {
		want = "edit"
	}
	for _, zone := range doc.Zones {
		for _, app := range zone.Apps {
			for _, action := range app.Actions {
				if action.Ext != extension || action.Name != want {
					continue
				}
				launch, err := url.Parse(officePlaceholder.ReplaceAllString(action.URL, ""))
				if err != nil || launch.Scheme != "https" || !strings.EqualFold(launch.Host, configured.Host) || launch.User != nil {
					return nil, proof, fmt.Errorf("office editor must use configured HTTPS origin")
				}
				return launch, proof, nil
			}
		}
	}
	return nil, proof, fmt.Errorf("office does not support this file type")
}

func (s *Server) handleWOPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/wopi/files/"), "/")
	if len(parts) > 2 || (len(parts) == 2 && parts[1] != "contents") {
		http.Error(w, "not found", 404)
		return
	}
	token := r.URL.Query().Get("access_token")
	if len(token) != 64 {
		http.Error(w, "unauthorized", 401)
		return
	}
	s.fileMu.Lock()
	session, ok := s.officeSessions[token]
	s.fileMu.Unlock()
	if !ok || session.Provider != "" || (s.cfgLive().Files.OfficeProvider != "" && s.cfgLive().Files.OfficeProvider != "collabora") || time.Now().After(session.Expires) || session.OfficeURL != strings.TrimRight(s.cfgLive().Files.OfficeURL, "/") {
		http.Error(w, "unauthorized", 401)
		return
	}
	if session.FileID != parts[0] {
		http.Error(w, "not found", 404)
		return
	}
	if session.Proof.Modulus != "" && !session.Proof.valid(r, token, s.publicBase()+r.URL.RequestURI(), time.Now()) {
		http.Error(w, "invalid office proof", 500)
		return
	}
	user, err := s.store.GetUserByID(r.Context(), session.User)
	owner, ownerErr := s.store.GetUserByID(r.Context(), session.Owner)
	if err != nil || ownerErr != nil || user == nil || owner == nil || !user.Enabled || !owner.Enabled {
		http.Error(w, "unauthorized", 401)
		return
	}
	writable := session.Writable
	if session.Owner != session.User {
		if !s.fileAccessAllowed(r, session.Owner, session.User, session.Path, false) {
			http.Error(w, "not found", 404)
			return
		}
		writable = writable && s.fileAccessAllowed(r, session.Owner, session.User, session.Path, true)
	}
	root, err := s.filesRoot(session.Owner)
	if err != nil {
		fileError(w, err)
		return
	}
	abs, err := safeJoin(root, session.Path)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "not found", 404)
		return
	}
	version := strings.Trim(fileETag(info), `"`)
	if r.Method == http.MethodGet {
		if len(parts) == 1 {
			writeJSON(w, 200, map[string]any{"BaseFileName": info.Name(), "OwnerId": session.Owner, "Size": info.Size(), "Version": version, "UserId": user.ID, "UserFriendlyName": user.DisplayName, "UserCanWrite": writable, "ReadOnly": !writable, "SupportsUpdate": true, "SupportsLocks": true, "SupportsGetLock": true, "UserCanNotWriteRelative": true, "LastModifiedTime": info.ModTime().UTC().Format(time.RFC3339Nano), "PostMessageOrigin": s.publicBase(), "IsAdminUser": false})
		} else {
			f, err := os.Open(abs)
			if err != nil {
				http.Error(w, "not found", 404)
				return
			}
			defer f.Close()
			w.Header().Set("X-WOPI-ItemVersion", version)
			w.Header().Set("Content-Type", "application/octet-stream")
			http.ServeContent(w, r, info.Name(), info.ModTime(), f)
		}
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if !writable {
		http.Error(w, "read-only document", 403)
		return
	}
	op := r.Header.Get("X-WOPI-Override")
	if len(parts) == 2 {
		if op != "PUT" {
			http.Error(w, "unsupported operation", 501)
			return
		}
		if info.Size() > 0 && r.Header.Get("X-WOPI-Lock") == "" && r.Header.Get("X-COOL-WOPI-Timestamp") == "" && r.Header.Get("X-LOOL-WOPI-Timestamp") == "" {
			w.Header().Set("X-WOPI-Lock", "")
			http.Error(w, "lock required", 409)
			return
		}
		temp, _, err := s.stageFile(r.Context(), session.Owner, r.Body, r.ContentLength)
		if err != nil {
			fileError(w, err)
			return
		}
		defer os.Remove(temp)
		s.fileMu.Lock()
		defer s.fileMu.Unlock()
		// Permissions may have been revoked while a large document was uploading.
		if session.Owner != session.User && !s.fileAccessAllowed(r, session.Owner, session.User, session.Path, true) {
			http.Error(w, "file access revoked", 403)
			return
		}
		// Collabora's timestamp extension protects updates when no WOPI lock is
		// used. Validate it against the current file, not the session's creation.
		stamp := r.Header.Get("X-COOL-WOPI-Timestamp")
		if stamp == "" {
			stamp = r.Header.Get("X-LOOL-WOPI-Timestamp")
		}
		current, statErr := os.Stat(abs)
		if statErr != nil {
			fileError(w, statErr)
			return
		}
		if stamp != "" {
			date, e := time.Parse(time.RFC3339Nano, stamp)
			if e != nil || !date.Equal(current.ModTime()) {
				w.Header().Set("X-WOPI-Lock", "")
				writeJSON(w, 409, map[string]int{"LOOLStatusCode": 1010, "COOLStatusCode": 1010})
				return
			}
		}
		locks := s.locksFor(session.Owner)
		lock := locks.office["/"+session.Path]
		if time.Now().After(lock.Expires) {
			lock = officeFileLock{}
		}
		if value := r.Header.Get("X-WOPI-Lock"); value != "" && (value != lock.Value || time.Now().After(lock.Expires)) {
			w.Header().Set("X-WOPI-Lock", lock.Value)
			http.Error(w, "lock mismatch", 409)
			return
		}
		release, err := s.claimFileWrite(session.Owner, session.Path, r.Header.Get("X-WOPI-Lock"))
		if err != nil {
			w.Header().Set("X-WOPI-Lock", lock.Value)
			http.Error(w, "document locked", 409)
			return
		}
		defer release()
		etag, err := s.commitFile(session.Owner, session.Path, temp, fileETag(info))
		if err != nil {
			w.Header().Set("X-WOPI-Lock", lock.Value)
			fileError(w, err)
			return
		}
		updated, _ := os.Stat(abs)
		w.Header().Set("X-WOPI-ItemVersion", strings.Trim(etag, `"`))
		writeJSON(w, 200, map[string]string{"LastModifiedTime": updated.ModTime().UTC().Format(time.RFC3339Nano)})
		return
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	ls := s.locksFor(session.Owner)
	key := "/" + session.Path
	lock := ls.office[key]
	now := time.Now()
	if now.After(lock.Expires) {
		delete(ls.office, key)
		lock = officeFileLock{}
	}
	value := r.Header.Get("X-WOPI-Lock")
	conflict := func() { w.Header().Set("X-WOPI-Lock", lock.Value); http.Error(w, "lock mismatch", 409) }
	if op == "GET_LOCK" {
		w.Header().Set("X-WOPI-Lock", lock.Value)
		w.WriteHeader(200)
		return
	}
	if value == "" || len(value) > 1024 {
		http.Error(w, "invalid lock", 400)
		return
	}
	switch op {
	case "LOCK", "REFRESH_LOCK":
		old := value
		if prior := r.Header.Get("X-WOPI-OldLock"); prior != "" {
			old = prior
		}
		if lock.Value != "" {
			if lock.Value != old {
				conflict()
				return
			}
			if _, err := ls.Refresh(now, lock.Token, 30*time.Minute); err != nil {
				conflict()
				return
			}
			lock.Value = value
			lock.Expires = now.Add(30 * time.Minute)
			ls.office[key] = lock
		} else {
			if op == "REFRESH_LOCK" || r.Header.Get("X-WOPI-OldLock") != "" {
				conflict()
				return
			}
			token, err := ls.Create(now, webdav.LockDetails{Root: key, Duration: 30 * time.Minute, ZeroDepth: true})
			if err != nil {
				conflict()
				return
			}
			ls.office[key] = officeFileLock{Value: value, Token: token, Expires: now.Add(30 * time.Minute)}
		}
	case "UNLOCK":
		if lock.Value != value {
			conflict()
			return
		}
		if err := ls.Unlock(now, lock.Token); err != nil {
			conflict()
			return
		}
		delete(ls.office, key)
	default:
		http.Error(w, "unsupported operation", 501)
		return
	}
	w.Header().Set("X-WOPI-ItemVersion", version)
	w.WriteHeader(200)
}
