package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

type fileActor struct {
	ID     string `json:"id,omitempty"`
	Source string `json:"source"`
}
type fileActivityEvent struct {
	ID           string    `json:"id"`
	At           time.Time `json:"at"`
	Action       string    `json:"action"`
	Object       string    `json:"object_id"`
	Path         string    `json:"path"`
	PreviousPath string    `json:"previous_path,omitempty"`
	Actor        fileActor `json:"actor"`
	Subject      string    `json:"subject,omitempty"`
	Rights       string    `json:"rights,omitempty"`
	// Bindings also retain identities of descendants when folders move or restore.
	Bindings map[string]string `json:"bindings,omitempty"`
	Affected []string          `json:"affected,omitempty"`
}
type fileActivityState struct {
	Paths  map[string]string
	Events []fileActivityEvent
}

// Caller holds fileMu. Truncate only an incomplete final record left by a crash.
func (s *Server) fileActivity(owner string) (*fileActivityState, error) {
	if s.fileActivities == nil {
		s.fileActivities = map[string]*fileActivityState{}
	}
	if state := s.fileActivities[owner]; state != nil {
		return state, nil
	}
	root, err := s.fileStateRoot(owner, "activity")
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(root, "events.jsonl"), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	state := &fileActivityState{Paths: map[string]string{}, Events: []fileActivityEvent{}}
	reader := bufio.NewReader(f)
	var offset int64
	for {
		line, e := reader.ReadBytes('\n')
		if e == io.EOF {
			if len(line) > 0 {
				if err = f.Truncate(offset); err != nil {
					return nil, err
				}
				if err = f.Sync(); err != nil {
					return nil, err
				}
			}
			break
		}
		if e != nil {
			return nil, e
		}
		var event fileActivityEvent
		if err = json.Unmarshal(line, &event); err != nil {
			return nil, err
		}
		if event.ID == "" || event.Object == "" {
			return nil, errors.New("invalid file activity record")
		}
		applyFileActivity(state, event)
		offset += int64(len(line))
	}
	s.fileActivities[owner] = state
	return state, nil
}
func applyFileActivity(state *fileActivityState, e fileActivityEvent) {
	for path, id := range e.Bindings {
		if id == "" {
			delete(state.Paths, path)
		} else {
			state.Paths[path] = id
		}
	}
	state.Events = append(state.Events, e)
}
func (s *Server) fileActivityBindings(owner, path string) map[string]string {
	state, err := s.fileActivity(owner)
	if err != nil {
		return nil
	}
	path = cleanRel(path)
	out := map[string]string{}
	for p, id := range state.Paths {
		if p == path || strings.HasPrefix(p, path+"/") {
			out[p] = id
		}
	}
	if out[path] == "" {
		out[path] = storage.NewID()
	}
	return out
}
func (s *Server) appendFileActivity(owner string, e fileActivityEvent) error {
	state, err := s.fileActivity(owner)
	if err != nil {
		return err
	}
	e.Path = cleanRel(e.Path)
	if e.Object == "" {
		e.Object = state.Paths[e.Path]
	}
	if e.Object == "" {
		e.Object = storage.NewID()
	}
	if e.Bindings == nil {
		e.Bindings = map[string]string{e.Path: e.Object}
	}
	if e.Actor.Source == "" {
		e.Actor.Source = "system"
	}
	if e.ID != "" {
		for _, prior := range state.Events {
			if prior.ID == e.ID {
				return nil
			}
		}
	} else {
		e.ID = storage.NewID()
	}
	e.At = time.Now().UTC()
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	root, err := s.fileStateRoot(owner, "activity")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(root, "events.jsonl"), os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	n, err := f.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = f.Sync()
	}
	if err != nil {
		_ = f.Truncate(info.Size())
		delete(s.fileActivities, owner)
		return err
	}
	applyFileActivity(state, e)
	return nil
}

// Activity is a user-facing feed, not a transaction log for file contents.
// A logging failure must not report an already committed write as failed.
func (s *Server) recordFileActivity(owner, path, action string, actors []fileActor) {
	actor := fileActor{Source: "system"}
	if len(actors) > 0 {
		actor = actors[0]
	}
	s.recordFileEvent(owner, fileActivityEvent{Path: path, Action: action, Actor: actor})
}
func (s *Server) recordFileEvent(owner string, e fileActivityEvent) {
	if err := s.appendFileActivity(owner, e); err != nil {
		s.log.Error("file activity persistence failed", "owner", owner, "action", e.Action, "error", err)
	}
	s.autoSpaceCheckpoint(owner, e.Action)
}
func (s *Server) requestFileActor(r *http.Request) fileActor {
	if strings.HasPrefix(r.URL.Path, "/s/") {
		return fileActor{Source: "public_link"}
	}
	if au, err := s.userFromBearer(r); err == nil {
		return fileActor{ID: au.ID, Source: "web"}
	}
	return fileActor{Source: "system"}
}
func (s *Server) recordFileMove(owner, from, to string, actors []fileActor) {
	bindings := s.fileActivityBindings(owner, from)
	if bindings == nil {
		return
	}
	from, to = cleanRel(from), cleanRel(to)
	id := bindings[from]
	out := map[string]string{}
	for p, v := range bindings {
		out[p] = ""
		out[to+strings.TrimPrefix(p, from)] = v
	}
	actor := fileActor{Source: "system"}
	if len(actors) > 0 {
		actor = actors[0]
	}
	s.recordFileEvent(owner, fileActivityEvent{Path: to, PreviousPath: from, Object: id, Action: "moved", Actor: actor, Bindings: out})
}
func pathAccessApplies(grant, path string) bool {
	return grant == path || strings.HasPrefix(path, grant+"/")
}

func (s *Server) handleFileCard(w http.ResponseWriter, r *http.Request, au *authUser) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	owner := r.URL.Query().Get("owner_id")
	if owner == "" {
		owner = au.ID
	}
	if len(owner) != 36 || strings.ContainsAny(owner, `/\.`) {
		writeJSON(w, 400, map[string]string{"error": "invalid owner"})
		return
	}
	path := cleanRel(r.URL.Query().Get("path"))
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	root, err := s.filesRoot(owner)
	if err != nil {
		fileError(w, err)
		return
	}
	abs, err := safeJoin(root, r.URL.Query().Get("path"))
	space, spaceErr := s.store.GetFileSpace(r.Context(), owner)
	if err != nil || (path == "" && spaceErr != nil) {
		fileError(w, errPathEscape)
		return
	}
	if owner != au.ID && !s.fileAccessAllowed(r, owner, au.ID, path, false) {
		writeJSON(w, 403, map[string]string{"error": "file access denied"})
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		fileError(w, err)
		return
	}
	grants, err := s.store.ListFileAccess(r.Context(), owner, "")
	if err != nil {
		fileError(w, err)
		return
	}
	access := []map[string]any{}
	effective := "read"
	for _, g := range grants {
		if !pathAccessApplies(g.Path, path) || (owner != au.ID && g.GranteeID != au.ID) {
			continue
		}
		if g.Path != path {
			parent, e := safeJoin(root, g.Path)
			if e != nil {
				continue
			}
			st, e := os.Stat(parent)
			if e != nil || !st.IsDir() {
				continue
			}
		}
		email := ""
		if u, e := s.store.GetUserByID(r.Context(), g.GranteeID); e == nil {
			email = u.Email
		}
		access = append(access, map[string]any{"path": g.Path, "grantee_id": g.GranteeID, "email": email, "rights": g.Rights, "inherited": g.Path != path})
		if g.GranteeID == au.ID && g.Rights == "write" {
			effective = "write"
		}
	}
	canManage := s.fileCanManage(r.Context(), owner, au.ID)
	if owner == au.ID {
		effective = "owner"
	} else if spaceErr == nil {
		role, _ := s.store.FileSpaceRights(r.Context(), owner, au.ID)
		if role == "manage" || role == "write" {
			effective = role
		}
	}
	result := map[string]any{"path": path, "name": info.Name(), "is_dir": info.IsDir(), "size": info.Size(), "mtime": info.ModTime(), "etag": fileETag(info), "owner_id": owner, "can_manage": canManage, "effective_rights": effective, "access": access}
	if spaceErr == nil {
		if !canManage {
			space.Groups = []storage.FileSpaceGroup{}
		}
		result["space"] = space
	}
	if canManage {
		shares, err := s.store.ListFileShares(r.Context(), owner)
		if err != nil {
			fileError(w, err)
			return
		}
		links := []map[string]any{}
		for _, sh := range shares {
			if !pathAccessApplies(sh.Path, path) {
				continue
			}
			active := (sh.ExpiresAt == nil || time.Now().Before(*sh.ExpiresAt)) && (sh.MaxDownloads == 0 || sh.DownloadCount < sh.MaxDownloads)
			links = append(links, map[string]any{"id": sh.ID, "path": sh.Path, "url": strings.TrimRight(s.publicBase(), "/") + "/s/" + sh.Token, "rights": sh.Rights, "inherited": sh.Path != path, "expires_at": sh.ExpiresAt, "password_protected": sh.PasswordHash != "", "max_downloads": sh.MaxDownloads, "download_count": sh.DownloadCount, "active": active})
		}
		result["shares"] = links
		state, err := s.fileActivity(owner)
		if err != nil {
			fileError(w, err)
			return
		}
		id := state.Paths[path]
		events := []map[string]any{}
		before := r.URL.Query().Get("before")
		start := len(state.Events) - 1
		if before != "" {
			start = -1
			for i, e := range state.Events {
				if e.ID == before {
					start = i - 1
					break
				}
			}

		}
		next := ""
		emails := map[string]string{}
		for i := start; i >= 0; i-- {
			e := state.Events[i]
			matches := id != "" && e.Object == id
			if !matches && id != "" {
				for _, v := range e.Affected {
					if v == id {
						matches = true
						break
					}
				}
			}
			if !matches && id != "" {
				for _, v := range e.Bindings {
					if v == id {
						matches = true
						break
					}
				}
			}
			if !matches {
				continue
			}
			if len(events) == 50 {
				next = state.Events[i+1].ID
				break
			}
			actorEmail := ""
			if e.Actor.ID != "" {
				actorEmail = emails[e.Actor.ID]
				if actorEmail == "" {
					if u, err := s.store.GetUserByID(context.Background(), e.Actor.ID); err == nil {
						actorEmail = u.Email
						emails[e.Actor.ID] = actorEmail
					}
				}
			}
			events = append(events, map[string]any{"id": e.ID, "at": e.At, "action": e.Action, "path": e.Path, "previous_path": e.PreviousPath, "actor": e.Actor, "actor_email": actorEmail, "subject": e.Subject, "rights": e.Rights})
		}
		// Cursor is the last returned event, independent of unrelated file activity.
		if next != "" && len(events) > 0 {
			next = events[len(events)-1]["id"].(string)
		}
		result["events"] = events
		result["next_cursor"] = next
	}
	writeJSON(w, 200, result)
}

func (s *Server) recordRevisionActivity(owner string, rev fileRevision, action string, actor fileActor, eventID string) {
	state, err := s.fileActivity(owner)
	if err != nil {
		s.log.Error("file activity unavailable", "error", err)
		return
	}
	id := rev.ActivityIDs[rev.Path]
	if id == "" && len(rev.ActivityIDs) == 1 {
		for _, value := range rev.ActivityIDs {
			id = value
		}
	}
	if id == "" {
		id = state.Paths[rev.Path]
	}
	if id == "" {
		return
	}
	affected := []string{}
	for _, v := range rev.ActivityIDs {
		affected = append(affected, v)
	}
	s.recordFileEvent(owner, fileActivityEvent{Affected: affected, ID: eventID, Path: rev.Path, Object: id, Action: action, Actor: actor, Subject: rev.ID, Bindings: map[string]string{}})
}
