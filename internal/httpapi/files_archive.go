package httpapi

import (
	"archive/zip"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

// Build the archive completely before sending headers, so an unreadable member
// produces an API error rather than a successful but truncated download.
func (s *Server) downloadFileArchive(w http.ResponseWriter, r *http.Request, root string) {
	abs, err := safeJoin(root, r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid path"})
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		writeJSON(w, 404, map[string]string{"error": "folder not found"})
		return
	}
	temp, err := os.CreateTemp("", "tayga-files-*.zip")
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "cannot create archive"})
		return
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	name := filepath.Base(abs)
	if abs == root {
		name = "files"
	}
	writer := zip.NewWriter(temp)
	err = filepath.WalkDir(abs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := r.Context().Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		inf, err := entry.Info()
		if err != nil {
			return err
		}
		if !inf.IsDir() && !inf.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		hdr, err := zip.FileInfoHeader(inf)
		if err != nil {
			return err
		}
		hdr.Name = name
		if rel != "." {
			hdr.Name += "/" + filepath.ToSlash(rel)
		}
		if inf.IsDir() {
			hdr.Name += "/"
		} else {
			hdr.Method = zip.Deflate
		}
		target, err := writer.CreateHeader(hdr)
		if err != nil {
			return err
		}
		if inf.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(target, file)
		return err
	})
	closeErr := writer.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "archive creation failed"})
		return
	}
	stat, err := temp.Stat()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "archive creation failed"})
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name + ".zip"}))
	http.ServeContent(w, r, name+".zip", stat.ModTime(), temp)
}
