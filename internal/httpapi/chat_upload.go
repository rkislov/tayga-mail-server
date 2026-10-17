package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/xmpp"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type chatUploadRecord struct {
	UserID, Path string
	Size         int64
	Expires      time.Time
}

func uploadToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func (s *Server) createChatUpload(ctx context.Context, userID, name string, size int64, contentType string) (xmpp.UploadSlot, error) {
	var slot xmpp.UploadSlot
	base := strings.TrimRight(s.publicBase(), "/")
	if !strings.HasPrefix(base, "https://") {
		return slot, fmt.Errorf("HTTPS public URL required")
	}
	if size < 0 || size > xmpp.MaxUploadSize || name == "" || len(name) > 255 || strings.ContainsAny(name, "/\\\r\n\x00") || name == "." || name == ".." {
		return slot, fmt.Errorf("invalid upload")
	}
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil || !user.Enabled {
		return slot, fmt.Errorf("user unavailable")
	}
	root, err := s.filesRoot(userID)
	if err != nil {
		return slot, err
	}
	token := uploadToken()
	rel := "chat/" + token[:16] + "/" + name
	dir := filepath.Join(s.ms.Root, "chat-upload-slots")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return slot, err
	}
	// Expired reservations hold no uploaded file and may be discarded.
	entries, _ := os.ReadDir(dir)
	pending := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir, entry.Name())
		raw, e := os.ReadFile(p)
		var record chatUploadRecord
		if e != nil || json.Unmarshal(raw, &record) != nil {
			continue
		}
		if time.Now().After(record.Expires) {
			os.Remove(p)
		} else if record.UserID == userID {
			pending++
		}
	}
	if pending >= 100 {
		return slot, fmt.Errorf("too many pending uploads")
	}
	abs, err := safeJoin(root, rel)
	if err != nil {
		return slot, err
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0750); err != nil {
		return slot, err
	}
	expiry := time.Now().Add(7 * 24 * time.Hour)
	share, err := s.store.CreateFileShare(ctx, &storage.FileShare{UserID: userID, Path: rel, Rights: "read", ExpiresAt: &expiry})
	if err != nil {
		return slot, err
	}
	record := chatUploadRecord{UserID: userID, Path: rel, Size: size, Expires: time.Now().Add(24 * time.Hour)}
	raw, _ := json.Marshal(record)
	if err = os.WriteFile(filepath.Join(dir, token+".json"), raw, 0600); err != nil {
		s.store.DeleteFileShare(ctx, userID, share.ID)
		return slot, err
	}
	slot.Put = base + "/chat-upload/" + token
	slot.Get = base + "/chat-download/" + share.Token + "/" + url.PathEscape(name)
	return slot, nil
}
func (s *Server) handleChatUploadSlot(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	var req struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
		Type string `json:"type"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
		w.WriteHeader(400)
		return
	}
	slot, err := s.createChatUpload(r.Context(), au.ID, req.Name, req.Size, req.Type)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, slot)
}
func (s *Server) handleChatUploadPut(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		w.WriteHeader(405)
		return
	}
	token := strings.TrimPrefix(r.URL.Path, "/chat-upload/")
	if len(token) != 64 {
		w.WriteHeader(404)
		return
	}
	if _, err := hex.DecodeString(token); err != nil {
		w.WriteHeader(404)
		return
	}
	recordPath := filepath.Join(s.ms.Root, "chat-upload-slots", token+".json")
	raw, err := os.ReadFile(recordPath)
	var record chatUploadRecord
	if err != nil || json.Unmarshal(raw, &record) != nil || time.Now().After(record.Expires) {
		w.WriteHeader(404)
		return
	}
	user, err := s.store.GetUserByID(r.Context(), record.UserID)
	if err != nil || !user.Enabled {
		w.WriteHeader(403)
		return
	}
	if r.ContentLength >= 0 && r.ContentLength != record.Size {
		w.WriteHeader(400)
		return
	}
	lock, err := os.OpenFile(recordPath+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		w.WriteHeader(409)
		return
	}
	lock.Close()
	defer os.Remove(recordPath + ".lock")
	root, err := s.filesRoot(record.UserID)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	dest, err := safeJoin(root, record.Path)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	if _, err = os.Stat(dest); !os.IsNotExist(err) {
		w.WriteHeader(409)
		return
	}
	temp, _, err := s.stageFile(r.Context(), record.UserID, http.MaxBytesReader(w, r.Body, record.Size+1), record.Size)
	if err != nil {
		fileError(w, err)
		return
	}
	defer os.Remove(temp)
	s.fileMu.Lock()
	_, err = s.commitFile(record.UserID, record.Path, temp, "")
	s.fileMu.Unlock()
	if err != nil {
		fileError(w, err)
		return
	}
	os.Remove(recordPath)
	w.WriteHeader(201)
}
func (s *Server) handleChatDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/chat-download/"), "/", 2)
	if len(parts) != 2 || parts[0] == "" {
		w.WriteHeader(404)
		return
	}
	clone := r.Clone(r.Context())
	copyURL := *r.URL
	clone.URL = &copyURL
	clone.URL.Path = "/s/" + parts[0]
	clone.Method = "GET"
	w.Header().Set("X-Content-Type-Options", "nosniff")
	s.handlePublicShare(w, clone)
}
