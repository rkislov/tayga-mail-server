package httpapi

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

var fileContentSearchSlots = make(chan struct{}, 4)

type fileSearchCacheEntry struct {
	Info os.FileInfo
	Text string
	Used time.Time
}
type fileSearchCandidate struct {
	Owner, Path string
	Info        os.FileInfo
	Match       string
	Snippet     *fileSearchSnippet
	Skipped     bool
	Empty       bool
}

func searchGrantsKey(grants []storage.FileAccess) string {
	parts := make([]string, 0, len(grants))
	for _, g := range grants {
		parts = append(parts, g.OwnerID+"\x00"+g.Path+"\x00"+g.Rights)
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}

// The cache never supplies candidates or authority. Each result is revalidated
// against the current file and permissions; old text cannot resurrect a file.
func (s *Server) fileSearchText(ctx context.Context, owner, path string, file *os.File, info os.FileInfo) (string, error) {
	key := owner + "\x00" + path
	s.fileSearchMu.Lock()
	cached, ok := s.fileSearchCache[key]
	if ok && os.SameFile(cached.Info, info) && fileETag(cached.Info) == fileETag(info) {
		cached.Used = time.Now()
		s.fileSearchCache[key] = cached
		s.fileSearchMu.Unlock()
		return cached.Text, nil
	}
	s.fileSearchMu.Unlock()
	text, err := extractFileSearchText(ctx, file, path)
	if err != nil {
		return "", err
	}
	s.fileSearchMu.Lock()
	defer s.fileSearchMu.Unlock()
	if s.fileSearchCache == nil {
		s.fileSearchCache = map[string]fileSearchCacheEntry{}
	}
	delete(s.fileSearchCache, key)
	for {
		bytes := len(text)
		oldest := ""
		var at time.Time
		for k, e := range s.fileSearchCache {
			bytes += len(e.Text)
			if oldest == "" || e.Used.Before(at) {
				oldest = k
				at = e.Used
			}
		}
		if len(s.fileSearchCache) < 64 && bytes <= 32<<20 {
			break
		}
		if oldest == "" {
			break
		}
		delete(s.fileSearchCache, oldest)
	}
	s.fileSearchCache[key] = fileSearchCacheEntry{Info: info, Text: text, Used: time.Now()}
	return text, nil
}
func (s *Server) searchFileContents(w http.ResponseWriter, r *http.Request, actor string) {
	w.Header().Set("Cache-Control", "no-store")
	query := normalizeFileSearchText(r.URL.Query().Get("q"))
	mode, scope := r.URL.Query().Get("mode"), r.URL.Query().Get("scope")
	if scope == "" {
		scope = "accessible"
	}
	if (mode != "content" && mode != "all" && mode != "name") || (scope != "own" && scope != "accessible") || query == "" || len(query) > 256 {
		writeJSON(w, 400, map[string]string{"error": "invalid search query, mode or scope"})
		return
	}
	select {
	case fileContentSearchSlots <- struct{}{}:
		defer func() { <-fileContentSearchSlots }()
	default:
		w.Header().Set("Retry-After", "2")
		writeJSON(w, 429, map[string]string{"error": "too many document searches; retry later"})
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 1000 {
		writeJSON(w, 400, map[string]string{"error": "invalid page"})
		return
	}
	base := r.URL.Query().Get("path")
	foldedQuery := foldFileSearchText(query)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	grants := []storage.FileAccess{}
	if scope == "accessible" || r.URL.Query().Get("owner_id") != "" {
		var err error
		grants, err = s.fileSearchGrants(ctx, actor)
		if err != nil {
			fileError(w, err)
			return
		}
	}
	type searchRoot struct{ Owner, Path string }
	rootOwner := actor
	if requested := r.URL.Query().Get("owner_id"); requested != "" {
		rootOwner = requested
	}
	roots := []searchRoot{{rootOwner, base}}
	owners := map[string]bool{actor: true}
	if scope == "accessible" && base == "" {
		for _, g := range grants {
			if !owners[g.OwnerID] {
				if !s.fileOwnerActive(ctx, g.OwnerID) {
					continue
				}
				owners[g.OwnerID] = true
			}
			roots = append(roots, searchRoot{g.OwnerID, g.Path})
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		if roots[i].Owner == actor && roots[j].Owner != actor {
			return true
		}
		if roots[j].Owner == actor && roots[i].Owner != actor {
			return false
		}
		if roots[i].Owner != roots[j].Owner {
			return roots[i].Owner < roots[j].Owner
		}
		return roots[i].Path < roots[j].Path
	})
	candidates := []fileSearchCandidate{}
	seen := map[string]bool{}
	visited, extracted := 0, 0
	truncated := false
	_, pdfErr := exec.LookPath("pdftotext")
	for _, source := range roots {
		if ctx.Err() != nil {
			truncated = true
			break
		}
		root, err := s.filesRoot(source.Owner)
		if err != nil {
			fileError(w, err)
			return
		}
		start, err := safeJoin(root, source.Path)
		if err != nil {
			if source.Owner == actor {
				fileError(w, err)
				return
			}
			continue
		}
		walkErr := filepath.WalkDir(start, func(abs string, entry fs.DirEntry, walkErr error) error {
			if ctx.Err() != nil {
				truncated = true
				return fs.SkipAll
			}
			visited++
			if visited > 100000 {
				truncated = true
				return fs.SkipAll
			}
			if walkErr != nil {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 || strings.HasPrefix(entry.Name(), ".upload-") {
				return nil
			}
			rel, err := filepath.Rel(root, abs)
			if err != nil || abs == root {
				return nil
			}
			rel = filepath.ToSlash(rel)
			key := source.Owner + "\x00" + rel
			if seen[key] {
				return nil
			}
			seen[key] = true
			info, err := entry.Info()
			if err != nil || (!info.IsDir() && !info.Mode().IsRegular()) {
				return nil
			}
			s.fileMu.Lock()
			allowed := source.Owner == actor || s.fileAccessAllowedContext(ctx, source.Owner, actor, rel, false)
			if !allowed {
				s.fileMu.Unlock()
				if info.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			candidate := fileSearchCandidate{Owner: source.Owner, Path: rel, Info: info}
			if mode != "content" && strings.Contains(foldFileSearchText(normalizeFileSearchText(entry.Name())), foldedQuery) {
				candidate.Match = "name"
				candidates = append(candidates, candidate)
				s.fileMu.Unlock()
				return nil
			}
			ext := strings.ToLower(filepath.Ext(rel))
			if mode == "name" || info.IsDir() || (ext != ".pdf" && ext != ".docx") {
				s.fileMu.Unlock()
				return nil
			}
			safe, err := safeJoin(root, rel)
			var file *os.File
			if err == nil {
				file, err = os.Open(safe)
			}
			if err == nil {
				candidate.Info, err = file.Stat()
			}
			s.fileMu.Unlock()
			if err != nil {
				if file != nil {
					file.Close()
				}
				return nil
			}
			defer file.Close()
			extracted++
			if extracted > 500 {
				truncated = true
				return fs.SkipAll
			}
			text, err := s.fileSearchText(ctx, source.Owner, rel, file, candidate.Info)
			if err != nil {
				candidate.Skipped = true
			} else if text == "" {
				candidate.Empty = true
			} else if snippet, found := contentSearchSnippet(text, foldedQuery); found {
				candidate.Match = "content"
				candidate.Snippet = &snippet
			}
			candidates = append(candidates, candidate)
			return nil
		})
		if walkErr != nil {
			fileError(w, walkErr)
			return
		}
		if truncated {
			break
		}
	}
	if err := r.Context().Err(); err != nil {
		return
	}
	if ctx.Err() != nil {
		truncated = true
	}
	// Hold the same mutex used by revocation through writing the response. Never
	// return a snapshot (including counters) if its sharing permissions changed.
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if _, err := s.userFromBearer(r); err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	if scope == "accessible" || rootOwner != actor {
		current, err := s.fileSearchGrants(r.Context(), actor)
		if err != nil {
			fileError(w, err)
			return
		}
		if searchGrantsKey(current) != searchGrantsKey(grants) {
			writeJSON(w, 409, map[string]string{"error": "file access changed; repeat search"})
			return
		}
	}
	for owner := range owners {
		if owner == actor {
			continue
		}
		if !s.fileOwnerActive(r.Context(), owner) {
			writeJSON(w, 409, map[string]string{"error": "file access changed; repeat search"})
			return
		}
	}
	matches := []map[string]any{}
	skipped, empty := 0, 0
	emails := map[string]string{}
	for _, candidate := range candidates {
		root, err := s.filesRoot(candidate.Owner)
		if err != nil {
			continue
		}
		abs, err := safeJoin(root, candidate.Path)
		if err != nil {
			continue
		}
		current, err := os.Stat(abs)
		if err != nil || !os.SameFile(candidate.Info, current) || fileETag(candidate.Info) != fileETag(current) {
			continue
		}
		if candidate.Owner != actor && !s.fileAccessAllowedContext(r.Context(), candidate.Owner, actor, candidate.Path, false) {
			continue
		}
		if candidate.Skipped {
			skipped++
		}
		if candidate.Empty {
			empty++
		}
		if candidate.Match == "" {
			continue
		}
		email := emails[candidate.Owner]
		if email == "" {
			email = s.fileOwnerLabel(r.Context(), candidate.Owner)
			emails[candidate.Owner] = email
		}
		rights := "read"
		if candidate.Owner == actor || s.fileAccessAllowedContext(r.Context(), candidate.Owner, actor, candidate.Path, true) {
			rights = "write"
		}
		space, spaceErr := s.store.GetFileSpace(r.Context(), candidate.Owner)
		spaceRights := ""
		if spaceErr == nil {
			spaceRights, _ = s.store.FileSpaceRights(r.Context(), space.ID, actor)
		}
		matches = append(matches, map[string]any{"space": spaceErr == nil, "space_rights": spaceRights, "owner_id": candidate.Owner, "owner_email": email, "shared": candidate.Owner != actor, "path": candidate.Path, "name": filepath.Base(candidate.Path), "is_dir": current.IsDir(), "size": current.Size(), "at": current.ModTime(), "rights": rights, "match": candidate.Match, "snippet": candidate.Snippet})
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i]["owner_id"].(string)+"\x00"+matches[i]["path"].(string) < matches[j]["owner_id"].(string)+"\x00"+matches[j]["path"].(string)
	})
	total := len(matches)
	from := (page - 1) * 50
	if from > total {
		from = total
	}
	to := from + 50
	if to > total {
		to = total
	}
	writeJSON(w, 200, map[string]any{"entries": matches[from:to], "total": total, "page": page, "page_size": 50, "truncated": truncated, "skipped_documents": skipped, "empty_documents": empty, "pdf_search_available": pdfErr == nil})
}

func (s *Server) fileSearchGrants(ctx context.Context, actor string) ([]storage.FileAccess, error) {
	grants, err := s.store.ListFileAccess(ctx, "", actor)
	if err != nil {
		return nil, err
	}
	spaces, err := s.store.ListFileSpaces(ctx, actor)
	if err != nil {
		return nil, err
	}
	for _, space := range spaces {
		grants = append(grants, storage.FileAccess{OwnerID: space.ID, Path: "", GranteeID: actor, Rights: space.Rights + ":" + fmt.Sprint(space.Revision)})
	}
	return grants, nil
}
