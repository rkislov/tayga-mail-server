// SPDX-License-Identifier: Apache-2.0
package httpapi

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

var errFileQuota = errors.New("file storage quota exceeded; remove files and empty trash or old versions")

type fileStorageUsage struct {
	Live     int64 `json:"live_bytes"`
	Trash    int64 `json:"trash_bytes"`
	Versions int64 `json:"version_bytes"`
	Used     int64 `json:"used_bytes"`
	Limit    int64 `json:"quota_bytes"`
}

// Quota measures logical retained content, including trash and every version.
// Upload staging has separate bounded session limits; it is checked again on commit.
// Caller holds fileMu so concurrent writers cannot overcommit available space.
func (s *Server) fileUsage(owner string) (fileStorageUsage, error) {
	var usage fileStorageUsage
	limit, err := s.fileQuotaLimit(owner)
	if err != nil {
		return usage, err
	}
	usage.Limit = limit
	root, err := s.filesRoot(owner)
	if err != nil {
		return usage, err
	}
	usage.Live, err = fileTreeBytes(root)
	if err != nil {
		return usage, err
	}
	for _, kind := range []string{"trash", "versions"} {
		root, err := s.fileStateRoot(owner, kind)
		if err != nil {
			return usage, err
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return usage, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			n, e := fileTreeBytes(filepath.Join(root, entry.Name(), "content"))
			if e != nil {
				return usage, e
			}
			if kind == "trash" {
				usage.Trash += n
			} else {
				usage.Versions += n
			}
		}
	}
	usage.Used = usage.Live + usage.Trash + usage.Versions
	return usage, nil
}
func fileTreeBytes(root string) (int64, error) {
	var size int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if info.Mode().IsRegular() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
func (s *Server) checkFileQuota(owner string, incoming int64) error {
	limit, err := s.fileQuotaLimit(owner)
	if err != nil {
		return err
	}
	if limit <= 0 {
		return nil
	}
	usage, err := s.fileUsage(owner)
	if err != nil {
		return err
	}
	if incoming > limit-usage.Used {
		return errFileQuota
	}
	return nil
}

func (s *Server) fileQuotaLimit(owner string) (int64, error) {
	if space, err := s.store.GetFileSpace(context.Background(), owner); err == nil {
		return space.QuotaBytes, nil
	}
	user, err := s.store.GetUserByID(context.Background(), owner)
	if err != nil {
		return 0, err
	}
	if user.ServiceClassID == "" {
		return 0, nil
	}
	class, err := s.store.GetServiceClass(context.Background(), user.ServiceClassID)
	if err != nil {
		return 0, err
	}
	return parseCoSConfig(class.Config).FileQuotaBytes, nil
}
