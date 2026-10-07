package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// Options controls what is included in an export.
type Options struct {
	TenantID    string // empty = all tenants
	IncludeMail bool
	MailRoot    string
}

// Manifest describes a Tayga backup archive.
type Manifest struct {
	Version    string    `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	TenantID   string    `json:"tenant_id,omitempty"`
	IncludeMail bool     `json:"include_mail"`
	Hostname   string    `json:"hostname,omitempty"`
}

// WriteTarGz writes a gzip-compressed tar backup to w.
func WriteTarGz(ctx context.Context, store storage.Driver, opts Options, w io.Writer) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	man := Manifest{
		Version:     "1",
		CreatedAt:   time.Now().UTC(),
		TenantID:    opts.TenantID,
		IncludeMail: opts.IncludeMail,
	}
	if err := writeJSON(tw, "manifest.json", man); err != nil {
		return err
	}

	var tenants []*storage.Tenant
	if opts.TenantID != "" {
		t, err := store.GetTenantByID(ctx, opts.TenantID)
		if err != nil {
			return err
		}
		tenants = []*storage.Tenant{t}
	} else {
		var err error
		tenants, err = store.ListTenants(ctx)
		if err != nil {
			return err
		}
	}
	if err := writeJSON(tw, "meta/tenants.json", tenants); err != nil {
		return err
	}

	var allDomains []*storage.Domain
	var allUsers []*storage.User
	for _, t := range tenants {
		domains, err := store.ListDomainsByTenant(ctx, t.ID)
		if err != nil {
			return err
		}
		allDomains = append(allDomains, domains...)
		users, err := store.ListUsersByTenant(ctx, t.ID)
		if err != nil {
			return err
		}
		allUsers = append(allUsers, users...)
	}
	if err := writeJSON(tw, "meta/domains.json", allDomains); err != nil {
		return err
	}
	// Export users without stripping password hashes (needed for restore).
	type userExport struct {
		ID           string `json:"id"`
		TenantID     string `json:"tenant_id"`
		DomainID     string `json:"domain_id"`
		Email        string `json:"email"`
		LocalPart    string `json:"local_part"`
		DisplayName  string `json:"display_name"`
		PasswordHash string `json:"password_hash"`
		AuthSource   string `json:"auth_source"`
		QuotaBytes   int64  `json:"quota_bytes"`
		Enabled      bool   `json:"enabled"`
	}
	ue := make([]userExport, 0, len(allUsers))
	for _, u := range allUsers {
		ue = append(ue, userExport{
			ID: u.ID, TenantID: u.TenantID, DomainID: u.DomainID,
			Email: u.Email, LocalPart: u.LocalPart, DisplayName: u.DisplayName,
			PasswordHash: u.PasswordHash, AuthSource: u.AuthSource,
			QuotaBytes: u.QuotaBytes, Enabled: u.Enabled,
		})
	}
	if err := writeJSON(tw, "meta/users.json", ue); err != nil {
		return err
	}

	for _, u := range allUsers {
		scripts, err := store.ListSieveScripts(ctx, u.ID)
		if err != nil {
			return err
		}
		for _, sc := range scripts {
			name := fmt.Sprintf("meta/sieve/%s/%s.sieve", u.ID, sanitize(sc.Name))
			meta := map[string]any{"name": sc.Name, "active": sc.Active, "user_id": u.ID, "email": u.Email}
			if err := writeJSON(tw, name+".json", meta); err != nil {
				return err
			}
			if err := writeBytes(tw, name, []byte(sc.Script)); err != nil {
				return err
			}
		}
		if opts.IncludeMail && opts.MailRoot != "" {
			ms := mailstore.New(opts.MailRoot)
			root := ms.UserRoot(u.Email)
			if st, err := os.Stat(root); err == nil && st.IsDir() {
				if err := addDir(tw, root, filepath.Join("mail", sanitize(u.Email))); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "..", "_")
	return s
}

func writeJSON(tw *tar.Writer, name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeBytes(tw, name, b)
}

func writeBytes(tw *tar.Writer, name string, b []byte) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0o640,
		Size:    int64(len(b)),
		ModTime: time.Now().UTC(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(b)
	return err
}

func addDir(tw *tar.Writer, src, prefix string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(prefix, rel))
		if info.IsDir() {
			if rel == "." {
				return nil
			}
			hdr := &tar.Header{Name: name + "/", Mode: 0o750, Typeflag: tar.TypeDir, ModTime: info.ModTime()}
			return tw.WriteHeader(hdr)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		hdr := &tar.Header{Name: name, Mode: 0o640, Size: info.Size(), ModTime: info.ModTime()}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = io.Copy(tw, f)
		return err
	})
}
