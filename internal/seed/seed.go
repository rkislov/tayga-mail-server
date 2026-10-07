package seed

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// Run creates a demo tenant/domain/user when seed.enabled is true.
func Run(ctx context.Context, cfg config.SeedConfig, store storage.Driver, hasher auth.PasswordHasher, ms *mailstore.Store, log *slog.Logger) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Domain == "" || cfg.Email == "" || cfg.Password == "" {
		return errors.New("seed requires domain, email, and password")
	}
	tenantName := cfg.Tenant
	if tenantName == "" {
		tenantName = "default"
	}

	tenant, err := store.GetTenantByName(ctx, tenantName)
	if errors.Is(err, storage.ErrNotFound) {
		tenant, err = store.CreateTenant(ctx, tenantName)
	}
	if err != nil {
		return err
	}

	domain, err := store.GetDomainByName(ctx, cfg.Domain)
	if errors.Is(err, storage.ErrNotFound) {
		domain, err = store.CreateDomain(ctx, tenant.ID, cfg.Domain)
	}
	if err != nil {
		return err
	}

	email := strings.ToLower(cfg.Email)
	if existing, err := store.GetUserByEmail(ctx, email); err == nil {
		if err := store.EnsureDAVDefaults(ctx, existing.ID); err != nil {
			return err
		}
		log.Info("seed user already exists", "email", email)
		return nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return err
	}

	hash, err := hasher.Hash(cfg.Password)
	if err != nil {
		return err
	}
	local := email
	if at := strings.LastIndex(email, "@"); at >= 0 {
		local = email[:at]
	}
	name := cfg.Name
	if name == "" {
		name = local
	}

	u, err := store.CreateUser(ctx, &storage.User{
		TenantID:     tenant.ID,
		DomainID:     domain.ID,
		Email:        email,
		LocalPart:    local,
		DisplayName:  name,
		PasswordHash: hash,
		AuthSource:   "local",
		Enabled:      true,
	})
	if err != nil {
		return err
	}

	root, err := ms.EnsureUser(u.Email)
	if err != nil {
		return err
	}
	if _, err := store.EnsureMailbox(ctx, u.ID, "INBOX", root); err != nil {
		return err
	}
	if err := store.EnsureDAVDefaults(ctx, u.ID); err != nil {
		return err
	}

	log.Info("seeded user", "email", email, "domain", domain.Name, "tenant", tenant.Name)
	return nil
}
