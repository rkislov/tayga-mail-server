package httpapi

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

type spaceDownloadTicket struct {
	Space, Point, Actor string
	Expires             time.Time
}
type recoveryReader struct {
	ctx context.Context
	r   io.Reader
}

func (r recoveryReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
func recoveryFileHash(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	_, err = io.Copy(hash, recoveryReader{ctx, f})
	return hex.EncodeToString(hash.Sum(nil)), err
}
func writeSpaceArchive(ctx context.Context, root string, point *spaceCheckpoint, writer io.Writer) error {
	data, err := json.Marshal(point)
	if err != nil {
		return err
	}
	var manifest spaceCheckpoint
	if err = json.Unmarshal(data, &manifest); err != nil {
		return err
	}
	gzipWriter := gzip.NewWriter(writer)
	tarWriter := tar.NewWriter(gzipWriter)
	for i := range manifest.Files {
		entry := &manifest.Files[i]
		if !checkpointPathAllowed(entry.Path) {
			return errPathEscape
		}
		path, err := safeJoin(root, entry.Path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || entry.Dir != info.IsDir() || (!entry.Dir && info.Size() != entry.Size) {
			return errFileConflict
		}
		header := &tar.Header{Name: entry.Path, Mode: 0600, Size: entry.Size, ModTime: info.ModTime()}
		if entry.Dir {
			header.Typeflag = tar.TypeDir
			header.Mode = 0700
			header.Size = 0
		}
		if err = tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if entry.Dir {
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, err = io.CopyN(io.MultiWriter(tarWriter, hash), recoveryReader{ctx, file}, entry.Size)
		file.Close()
		if err != nil {
			return err
		}
		entry.SHA256 = hex.EncodeToString(hash.Sum(nil))
	}
	data, err = json.Marshal(manifest)
	if err != nil {
		return err
	}
	if len(data) > recoveryManifestLimit {
		return errFileTooLarge
	}
	if err = tarWriter.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(data)), ModTime: time.Now().UTC()}); err == nil {
		_, err = tarWriter.Write(data)
	}
	if err != nil {
		return err
	}
	if err = tarWriter.Close(); err != nil {
		return err
	}
	return gzipWriter.Close()
}
func (s *Server) importSpaceArchive(ctx context.Context, reader io.Reader) (string, *spaceCheckpoint, error) {
	root := filepath.Join(s.ms.Root, "space-recovery", "imports")
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", nil, err
	}
	temp, err := os.MkdirTemp(root, "import-")
	if err != nil {
		return "", nil, err
	}
	success := false
	defer func() {
		if !success {
			os.RemoveAll(temp)
		}
	}()
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return "", nil, err
	}
	defer gzipReader.Close()
	limit := &io.LimitedReader{R: recoveryReader{ctx, gzipReader}, N: recoveryMaxBytes + recoveryManifestLimit + int64(recoveryMaxEntries)*2048 + 1}
	archive := tar.NewReader(limit)
	observed := map[string]recoveryEntry{}
	var point *spaceCheckpoint
	var total int64
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, err
		}
		if point != nil {
			return "", nil, errors.New("archive data after manifest")
		}
		if header.Name == "manifest.json" {
			if header.Typeflag != tar.TypeReg || header.Size > recoveryManifestLimit {
				return "", nil, storage.ErrFileSpaceInvalid
			}
			var value spaceCheckpoint
			decoder := json.NewDecoder(io.LimitReader(archive, recoveryManifestLimit+1))
			decoder.DisallowUnknownFields()
			if err = decoder.Decode(&value); err != nil {
				return "", nil, err
			}
			point = &value
			continue
		}
		if !checkpointPathAllowed(header.Name) || observed[header.Name].Path != "" || len(observed) >= recoveryMaxEntries || (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir) || header.Size < 0 || header.Size > maxWebUploadBytes {
			return "", nil, storage.ErrFileSpaceInvalid
		}
		dir := header.Typeflag == tar.TypeDir
		total += header.Size
		if total > recoveryMaxBytes {
			return "", nil, errFileTooLarge
		}
		if strings.HasPrefix(header.Name, "state/") && !recoveryContentPath(header.Name) && header.Size > 64<<20 {
			return "", nil, errFileTooLarge
		}
		path, err := safeJoin(temp, header.Name)
		if err != nil {
			return "", nil, err
		}
		entry := recoveryEntry{Path: header.Name, Dir: dir, Size: header.Size}
		if dir {
			if header.Size != 0 {
				return "", nil, storage.ErrFileSpaceInvalid
			}
			err = os.MkdirAll(path, 0700)
		} else {
			if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return "", nil, err
			}
			file, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if e != nil {
				return "", nil, e
			}
			hash := sha256.New()
			_, err = io.CopyN(io.MultiWriter(file, hash), archive, header.Size)
			if err == nil {
				err = file.Sync()
			}
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
			entry.SHA256 = hex.EncodeToString(hash.Sum(nil))
			if err == nil {
				err = os.Chtimes(path, header.ModTime, header.ModTime)
			}
		}
		if err != nil {
			return "", nil, err
		}
		observed[entry.Path] = entry
	}
	// Consume the gzip trailer, validating CRC and rejecting hidden trailing data.
	tail, err := io.ReadAll(io.LimitReader(limit, (1<<20)+1))
	if err != nil || len(tail) > 1<<20 || limit.N <= 0 {
		return "", nil, errors.New("invalid archive trailer")
	}
	for _, b := range tail {
		if b != 0 {
			return "", nil, errors.New("unexpected archive trailer")
		}
	}
	if point == nil || point.Format != 1 || point.State == nil || point.State.Space == nil || point.Info.SpaceID != point.State.Space.ID || !storage.ValidFileSpaceID(point.Info.SpaceID) || point.Info.At.IsZero() || len(point.Info.Label) > 200 || len(point.Files) != len(observed) {
		return "", nil, storage.ErrFileSpaceInvalid
	}
	seen := map[string]bool{}
	for _, entry := range point.Files {
		actual, ok := observed[entry.Path]
		if !ok || seen[entry.Path] || entry.Dir != actual.Dir || entry.Size != actual.Size || (!entry.Dir && (len(entry.SHA256) != 64 || entry.SHA256 != actual.SHA256)) {
			return "", nil, errors.New("archive checksum or file list mismatch")
		}
		seen[entry.Path] = true
	}
	for _, kind := range []string{"files", "state"} {
		if entry, ok := observed[kind]; !ok || !entry.Dir {
			return "", nil, storage.ErrFileSpaceInvalid
		}
	}
	if err = validateRecoveryMetadata(temp); err != nil {
		return "", nil, err
	}
	point.Info.ID = storage.NewID()
	point.Info.Automatic = false
	point.Info.Label = "Imported backup"
	point.Info.Entries = len(observed)
	point.Info.Bytes = total
	if err = writeFileStateJSON(filepath.Join(temp, "checkpoint.json"), point); err != nil {
		return "", nil, err
	}
	success = true
	return temp, point, nil
}
func recoveryContentPath(path string) bool {
	parts := strings.Split(path, "/")
	return len(parts) >= 4 && parts[0] == "state" && (parts[1] == "versions" || parts[1] == "trash") && parts[3] == "content"
}
func validateRecoveryMetadata(root string) error {
	policy, err := os.ReadFile(filepath.Join(root, "state", "retention", "policy.json"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		var p fileRetentionPolicy
		if json.Unmarshal(policy, &p) != nil || !p.valid() {
			return storage.ErrFileSpaceInvalid
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "state", "cleanup"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return storage.ErrFileSpaceInvalid
		}
		data, err := os.ReadFile(filepath.Join(root, "state", "cleanup", entry.Name()))
		if err != nil {
			return err
		}
		var e fileCleanupEntry
		if json.Unmarshal(data, &e) != nil || !storage.ValidFileSpaceID(e.ID) || e.ID+".json" != entry.Name() || !storage.ValidFileSpaceID(e.Revision.ID) || (e.Kind != "versions" && e.Kind != "trash") || e.Bytes < 0 || !checkpointPathAllowed("files/"+e.Revision.Path) {
			return storage.ErrFileSpaceInvalid
		}
	}
	for _, kind := range []string{"versions", "trash"} {
		base := filepath.Join(root, "state", kind)
		dirs, err := os.ReadDir(base)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, dir := range dirs {
			if !dir.IsDir() || !storage.ValidFileSpaceID(dir.Name()) {
				return storage.ErrFileSpaceInvalid
			}
			path := filepath.Join(base, dir.Name(), "metadata.json")
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if len(data) > 16<<20 {
				return errFileTooLarge
			}
			var rev fileRevision
			if json.Unmarshal(data, &rev) != nil || rev.ID != dir.Name() || rev.Path == "" || rev.Size < 0 || !checkpointPathAllowed("files/"+rev.Path) {
				return storage.ErrFileSpaceInvalid
			}
		}
	}
	// A full JSONL line must be valid; no unbounded line can be introduced on import.
	path := filepath.Join(root, "state", "activity", "events.jsonl")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) > 64<<20 {
		return errFileTooLarge
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var event fileActivityEvent
		if json.Unmarshal([]byte(line), &event) != nil || !storage.ValidFileSpaceID(event.ID) || !storage.ValidFileSpaceID(event.Object) {
			return fmt.Errorf("invalid activity record")
		}
	}
	return nil
}
