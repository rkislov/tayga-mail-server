package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const maxWebUploadBytes int64 = 10 << 30

// Write beside the destination and rename only after a complete upload.
func streamUpload(w http.ResponseWriter, r *http.Request, dest string) (int64, error) {
	if r.ContentLength > maxWebUploadBytes {
		return 0, fmt.Errorf("file exceeds 10 GiB")
	}
	f, err := os.CreateTemp(filepath.Dir(dest), ".upload-*")
	if err != nil {
		return 0, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	n, err := io.Copy(f, http.MaxBytesReader(w, r.Body, maxWebUploadBytes))
	if err != nil {
		return n, err
	}
	if err = f.Chmod(0o640); err != nil {
		return n, err
	}
	if err = f.Close(); err != nil {
		return n, err
	}
	return n, os.Rename(f.Name(), dest)
}
