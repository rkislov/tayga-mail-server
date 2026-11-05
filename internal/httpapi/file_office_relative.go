package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

func (s *Server) officeCanWriteRelative(r *http.Request, session officeSession) bool {
	if !session.Writable {
		return false
	}
	return session.Owner == session.User || s.fileAccessAllowed(r, session.Owner, session.User, cleanRel(filepath.Dir(session.Path)), true)
}

// WOPI uses standard UTF-7, not IMAP's modified UTF-7, for target headers.
func decodeWOPITarget(value string) (string, error) {
	if len(value) > 4096 {
		return "", fmt.Errorf("target name too long")
	}
	var out strings.Builder
	for i := 0; i < len(value); {
		if value[i] < 32 || value[i] > 126 {
			return "", fmt.Errorf("target must be UTF-7")
		}
		if value[i] != '+' {
			out.WriteByte(value[i])
			i++
			continue
		}
		end := i + 1
		if end < len(value) && value[end] == '-' {
			out.WriteByte('+')
			i = end + 1
			continue
		}
		for end < len(value) && strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", rune(value[end])) {
			end++
		}
		if end == i+1 {
			return "", fmt.Errorf("invalid UTF-7 shift")
		}
		data, err := base64.RawStdEncoding.Strict().DecodeString(value[i+1 : end])
		if err != nil || len(data)%2 != 0 {
			return "", fmt.Errorf("invalid UTF-7 data")
		}
		units := make([]uint16, len(data)/2)
		for j := range units {
			units[j] = binary.BigEndian.Uint16(data[j*2:])
		}
		for j := 0; j < len(units); j++ {
			if units[j] >= 0xd800 && units[j] <= 0xdbff {
				if j+1 >= len(units) || units[j+1] < 0xdc00 || units[j+1] > 0xdfff {
					return "", fmt.Errorf("invalid UTF-7 surrogate")
				}
				j++
			} else if units[j] >= 0xdc00 && units[j] <= 0xdfff {
				return "", fmt.Errorf("invalid UTF-7 surrogate")
			}
		}
		out.WriteString(string(utf16.Decode(units)))
		i = end
		if i < len(value) && value[i] == '-' {
			i++
		}
	}
	return out.String(), nil
}
func validOfficeTarget(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > 255 || strings.TrimSpace(name) != name || strings.TrimRight(name, " .") != name {
		return false
	}
	for _, r := range name {
		if r < 32 || r == 127 || strings.ContainsRune(`/\<>:"|?*`, r) {
			return false
		}
	}
	return true
}
func suggestedOfficeTarget(name, source string) string {
	if strings.HasPrefix(name, ".") {
		name = strings.TrimSuffix(filepath.Base(source), filepath.Ext(source)) + name
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || strings.ContainsRune(`/\<>:"|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" {
		name = "Document"
	}
	if len(name) > 220 {
		ext := filepath.Ext(name)
		if len(ext) > 40 {
			ext = ""
		}
		base := strings.TrimSuffix(name, ext)
		base = base[:220-len(ext)]
		for !utf8.ValidString(base) {
			base = base[:len(base)-1]
		}
		name = base + ext
	}
	return name
}
func (s *Server) handleWOPIPutRelative(w http.ResponseWriter, r *http.Request, token string, session officeSession) {
	if !s.officeCanWriteRelative(r, session) {
		http.Error(w, "cannot create relative file", 501)
		return
	}
	specific, suggested := r.Header.Get("X-WOPI-RelativeTarget"), r.Header.Get("X-WOPI-SuggestedTarget")
	if (specific == "") == (suggested == "") {
		http.Error(w, "one target header required", 400)
		return
	}
	raw := specific
	if suggested != "" {
		raw = suggested
	}
	name, err := decodeWOPITarget(raw)
	if err != nil {
		http.Error(w, "invalid target encoding", 400)
		return
	}
	if suggested != "" {
		name = suggestedOfficeTarget(name, session.Path)
	} else if !validOfficeTarget(name) {
		http.Error(w, "invalid target name", 400)
		return
	}
	overwrite := specific != "" && strings.EqualFold(r.Header.Get("X-WOPI-OverwriteRelativeTarget"), "true")
	declared := int64(-1)
	if v := r.Header.Get("X-WOPI-Size"); v != "" {
		declared, err = strconv.ParseInt(v, 10, 64)
		if err != nil || declared < 0 {
			http.Error(w, "invalid size", 400)
			return
		}
		if declared > maxWebUploadBytes {
			fileError(w, errFileTooLarge)
			return
		}
	}
	temp, size, err := s.stageFile(r.Context(), session.Owner, r.Body, r.ContentLength)
	if err != nil {
		fileError(w, err)
		return
	}
	defer os.Remove(temp)
	if declared >= 0 && declared != size {
		fileError(w, errFileIncomplete)
		return
	}
	var entropy [32]byte
	if _, err = rand.Read(entropy[:]); err != nil {
		fileError(w, err)
		return
	}
	newToken := hex.EncodeToString(entropy[:])
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	current, ok := s.officeSessions[token]
	if !ok || current != session || time.Now().After(session.Expires) || session.OfficeURL != strings.TrimRight(s.cfgLive().Files.OfficeURL, "/") || (s.cfgLive().Files.OfficeProvider != "" && s.cfgLive().Files.OfficeProvider != "collabora") {
		http.Error(w, "office session expired", 401)
		return
	}
	for _, id := range []string{session.Owner, session.User} {
		user, e := s.store.GetUserByID(r.Context(), id)
		if e != nil || !user.Enabled {
			http.Error(w, "unauthorized", 401)
			return
		}
	}
	if !s.officeCanWriteRelative(r, session) || (session.Owner != session.User && !s.fileAccessAllowed(r, session.Owner, session.User, session.Path, false)) {
		http.Error(w, "file access revoked", 501)
		return
	}
	root, err := s.filesRoot(session.Owner)
	if err != nil {
		fileError(w, err)
		return
	}
	source, err := safeJoin(root, session.Path)
	if err != nil {
		fileError(w, err)
		return
	}
	if info, e := os.Stat(source); e != nil || !info.Mode().IsRegular() {
		http.Error(w, "source not found", 404)
		return
	}
	parent := cleanRel(filepath.Dir(session.Path))
	var target string
	var info os.FileInfo
	for n := 0; n < 10000; n++ {
		candidate := name
		if n > 0 {
			ext := filepath.Ext(name)
			candidate = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), n, ext)
		}
		target = cleanRel(filepath.ToSlash(filepath.Join(parent, candidate)))
		abs, e := safeJoin(root, target)
		if e != nil {
			fileError(w, e)
			return
		}
		info, err = os.Stat(abs)
		if err != nil && !os.IsNotExist(err) {
			fileError(w, err)
			return
		}
		if suggested != "" && (info != nil || target == session.Path) {
			continue
		}
		name = candidate
		break
	}
	if suggested != "" && info != nil {
		http.Error(w, "cannot allocate target name", 500)
		return
	}
	conflict := func(message string) {
		lock := s.locksFor(session.Owner).office["/"+target]
		value := ""
		if time.Now().Before(lock.Expires) {
			value = lock.Value
		}
		w.Header().Set("X-WOPI-Lock", value)
		http.Error(w, message, 409)
	}
	if target == session.Path {
		conflict("target must differ from source")
		return
	}
	if info != nil && (!overwrite || !info.Mode().IsRegular()) {
		conflict("target already exists")
		return
	}
	if session.Owner != session.User && !s.fileAccessAllowed(r, session.Owner, session.User, target, true) {
		http.Error(w, "target access denied", 501)
		return
	}
	// Reserve session capacity before changing the filesystem. A returned URL is
	// scoped to the new file and the original actor, with the original expiry.
	count := 0
	for key, value := range s.officeSessions {
		if time.Now().After(value.Expires) {
			delete(s.officeSessions, key)
		} else if value.User == session.User {
			count++
		}
	}
	if count >= 64 || len(s.officeSessions) >= 4096 {
		http.Error(w, "too many open documents", 429)
		return
	}
	release, err := s.claimFileWrite(session.Owner, target, "")
	if err != nil {
		if suggested != "" {
			http.Error(w, "target collection locked", http.StatusLocked)
		} else {
			conflict("target locked")
		}
		return
	}
	defer release()
	if r.Context().Err() != nil {
		fileError(w, r.Context().Err())
		return
	}
	etag, err := s.commitFile(session.Owner, target, temp, fileETag(info))
	if err != nil {
		fileError(w, err)
		return
	}
	next := session
	next.Path, next.FileID, next.ETag = target, officeFileID(session.Owner, target), etag
	s.officeSessions[newToken] = next
	w.Header().Set("X-WOPI-ItemVersion", strings.Trim(etag, `"`))
	writeJSON(w, 200, map[string]string{"Name": name, "Url": s.publicBase() + "/wopi/files/" + next.FileID + "?access_token=" + url.QueryEscape(newToken)})
}
