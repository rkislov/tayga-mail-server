package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// RestoreOptions controls restore behaviour.
type RestoreOptions struct {
	MailRoot    string
	IncludeMail bool
	// SkipExisting leaves users/domains that already exist untouched.
	SkipExisting bool
}

// RestoreReport summarizes restore results.
type RestoreReport struct {
	TenantsCreated  int `json:"tenants_created"`
	DomainsCreated  int `json:"domains_created"`
	UsersCreated    int `json:"users_created"`
	UsersSkipped    int `json:"users_skipped"`
	ScriptsRestored int `json:"scripts_restored"`
	MailFiles       int `json:"mail_files"`
}

// UserExport is the on-disk user record in backups.
type UserExport struct {
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

type sieveMeta struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// RestoreTarGz reads a backup archive and applies it to store / maildir.
func RestoreTarGz(ctx context.Context, store storage.Driver, r io.Reader, opts RestoreOptions) (*RestoreReport, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	files := map[string][]byte{}
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		name := filepath.ToSlash(hdr.Name)
		data, err := io.ReadAll(io.LimitReader(tr, 64<<20))
		if err != nil {
			return nil, err
		}
		files[name] = data
	}

	rep := &RestoreReport{}
	var tenants []*storage.Tenant
	if err := json.Unmarshal(files["meta/tenants.json"], &tenants); err != nil {
		return nil, fmt.Errorf("tenants.json: %w", err)
	}
	var domains []*storage.Domain
	if err := json.Unmarshal(files["meta/domains.json"], &domains); err != nil {
		return nil, fmt.Errorf("domains.json: %w", err)
	}
	var users []UserExport
	if err := json.Unmarshal(files["meta/users.json"], &users); err != nil {
		return nil, fmt.Errorf("users.json: %w", err)
	}

	tenantMap := map[string]string{} // oldID -> effective ID
	for _, t := range tenants {
		if t == nil || t.ID == "" {
			continue
		}
		if existing, err := store.GetTenantByID(ctx, t.ID); err == nil {
			tenantMap[t.ID] = existing.ID
			continue
		}
		if existing, err := store.GetTenantByName(ctx, t.Name); err == nil {
			tenantMap[t.ID] = existing.ID
			continue
		}
		if _, err := store.InsertTenant(ctx, t); err != nil {
			return rep, fmt.Errorf("tenant %s: %w", t.Name, err)
		}
		tenantMap[t.ID] = t.ID
		rep.TenantsCreated++
	}

	domainMap := map[string]string{}
	for _, d := range domains {
		if d == nil || d.ID == "" {
			continue
		}
		tid := tenantMap[d.TenantID]
		if tid == "" {
			tid = d.TenantID
		}
		d.TenantID = tid
		if existing, err := store.GetDomainByID(ctx, d.ID); err == nil {
			domainMap[d.ID] = existing.ID
			continue
		}
		if existing, err := store.GetDomainByName(ctx, d.Name); err == nil {
			domainMap[d.ID] = existing.ID
			continue
		}
		if _, err := store.InsertDomain(ctx, d); err != nil {
			return rep, fmt.Errorf("domain %s: %w", d.Name, err)
		}
		domainMap[d.ID] = d.ID
		rep.DomainsCreated++
	}

	userIDs := map[string]string{}
	for _, ue := range users {
		if existing, err := store.GetUserByEmail(ctx, ue.Email); err == nil {
			userIDs[ue.ID] = existing.ID
			rep.UsersSkipped++
			if !opts.SkipExisting {
				_ = store.UpdateUserPassword(ctx, existing.ID, ue.PasswordHash)
				_ = store.UpdateUserQuota(ctx, existing.ID, ue.QuotaBytes)
				_ = store.UpdateUserEnabled(ctx, existing.ID, ue.Enabled)
				_ = store.UpdateUserProfile(ctx, existing.ID, ue.DisplayName)
			}
			continue
		}
		tid := tenantMap[ue.TenantID]
		if tid == "" {
			tid = ue.TenantID
		}
		did := domainMap[ue.DomainID]
		if did == "" {
			did = ue.DomainID
		}
		u := &storage.User{
			ID: ue.ID, TenantID: tid, DomainID: did,
			Email: ue.Email, LocalPart: ue.LocalPart, DisplayName: ue.DisplayName,
			PasswordHash: ue.PasswordHash, AuthSource: ue.AuthSource,
			QuotaBytes: ue.QuotaBytes, Enabled: ue.Enabled,
		}
		if _, err := store.CreateUser(ctx, u); err != nil {
			return rep, fmt.Errorf("user %s: %w", ue.Email, err)
		}
		userIDs[ue.ID] = u.ID
		rep.UsersCreated++
		if opts.MailRoot != "" {
			ms := mailstore.New(opts.MailRoot)
			_, _ = ms.EnsureUser(ue.Email)
			_ = store.EnsureDAVDefaults(ctx, u.ID)
			root, _ := ms.EnsureUser(ue.Email)
			_, _ = store.EnsureMailbox(ctx, u.ID, "INBOX", root)
		}
	}

	// Sieve scripts: meta/sieve/{userID}/{name}.sieve + .json
	for name, data := range files {
		if !strings.HasPrefix(name, "meta/sieve/") || !strings.HasSuffix(name, ".sieve") {
			continue
		}
		metaName := name + ".json"
		metaBytes, ok := files[metaName]
		if !ok {
			continue
		}
		var meta sieveMeta
		if err := json.Unmarshal(metaBytes, &meta); err != nil {
			continue
		}
		uid := userIDs[meta.UserID]
		if uid == "" {
			uid = meta.UserID
		}
		if _, err := store.PutSieveScript(ctx, uid, meta.Name, string(data)); err != nil {
			return rep, fmt.Errorf("sieve %s: %w", meta.Name, err)
		}
		if meta.Active {
			_ = store.SetActiveSieveScript(ctx, uid, meta.Name)
		}
		rep.ScriptsRestored++
	}

	if opts.IncludeMail && opts.MailRoot != "" {
		ms := mailstore.New(opts.MailRoot)
		emailToUID := map[string]string{}
		for _, ue := range users {
			uid := userIDs[ue.ID]
			if uid == "" {
				if u, err := store.GetUserByEmail(ctx, ue.Email); err == nil {
					uid = u.ID
				}
			}
			if uid != "" {
				emailToUID[sanitize(ue.Email)] = uid
			}
		}
		touched := map[string]string{}
		for name, data := range files {
			if !strings.HasPrefix(name, "mail/") {
				continue
			}
			rel := strings.TrimPrefix(name, "mail/")
			parts := strings.SplitN(rel, "/", 2)
			if len(parts) < 2 {
				continue
			}
			email := parts[0]
			rest := parts[1]
			destRoot := ms.UserRoot(email)
			dest := filepath.Join(destRoot, filepath.FromSlash(rest))
			if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
				return rep, err
			}
			if err := os.WriteFile(dest, data, 0o640); err != nil {
				return rep, err
			}
			rep.MailFiles++
			if uid := emailToUID[email]; uid != "" {
				touched[email] = uid
			}
		}
		for email, uid := range touched {
			if _, err := indexUserMaildir(ctx, store, ms, uid, email); err != nil {
				return rep, err
			}
		}
	}

	return rep, nil
}

func indexUserMaildir(ctx context.Context, store storage.Driver, ms *mailstore.Store, userID, email string) (int, error) {
	root, err := ms.EnsureUser(email)
	if err != nil {
		return 0, err
	}
	mb, err := store.EnsureMailbox(ctx, userID, "INBOX", root)
	if err != nil {
		return 0, err
	}
	existing, _ := store.ListMessages(ctx, mb.ID)
	known := map[string]struct{}{}
	for _, m := range existing {
		known[m.FilePath] = struct{}{}
	}
	n := 0
	for _, sub := range []string{"cur", "new"} {
		dir := filepath.Join(root, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			abs := filepath.Join(dir, e.Name())
			rel, err := filepath.Rel(ms.Root, abs)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			if _, ok := known[rel]; ok {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			flags := ""
			if sub == "cur" {
				flags = `\Seen`
			}
			if _, err := store.InsertMessage(ctx, &storage.Message{
				MailboxID:    mb.ID,
				Size:         info.Size(),
				Flags:        flags,
				InternalDate: info.ModTime().UTC(),
				FilePath:     rel,
			}); err != nil {
				return n, err
			}
			n++
		}
	}
	// Also index Maildir++ folders (.Sent, …)
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), ".") {
			continue
		}
		folder := strings.TrimPrefix(e.Name(), ".")
		folderPath := filepath.Join(root, e.Name())
		fmb, err := store.EnsureMailbox(ctx, userID, folder, folderPath)
		if err != nil {
			continue
		}
		existing, _ := store.ListMessages(ctx, fmb.ID)
		known := map[string]struct{}{}
		for _, m := range existing {
			known[m.FilePath] = struct{}{}
		}
		for _, sub := range []string{"cur", "new"} {
			dir := filepath.Join(folderPath, sub)
			ents, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, fe := range ents {
				if fe.IsDir() {
					continue
				}
				abs := filepath.Join(dir, fe.Name())
				rel, err := filepath.Rel(ms.Root, abs)
				if err != nil {
					continue
				}
				rel = filepath.ToSlash(rel)
				if _, ok := known[rel]; ok {
					continue
				}
				info, err := fe.Info()
				if err != nil {
					continue
				}
				flags := ""
				if sub == "cur" {
					flags = `\Seen`
				}
				if _, err := store.InsertMessage(ctx, &storage.Message{
					MailboxID: fmb.ID, Size: info.Size(), Flags: flags,
					InternalDate: info.ModTime().UTC(), FilePath: rel,
				}); err != nil {
					return n, err
				}
				n++
			}
		}
	}
	return n, nil
}
