package seed

import (
	"context"
	"errors"
	"fmt"
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
	email := strings.ToLower(strings.TrimSpace(cfg.Email))
	seedDomain := strings.ToLower(strings.TrimSpace(cfg.Domain))
	emailDomain := storage.DomainOfEmail(email)
	if emailDomain == "" {
		return fmt.Errorf("seed: email %q has no domain part", email)
	}
	if emailDomain != seedDomain {
		return fmt.Errorf("seed: email domain %q does not match seed.domain %q (fix typo in /etc/tayga/tayga.yaml)", emailDomain, seedDomain)
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

	domain, err := store.GetDomainByName(ctx, seedDomain)
	if errors.Is(err, storage.ErrNotFound) {
		domain, err = store.CreateDomain(ctx, tenant.ID, seedDomain)
	}
	if err != nil {
		return err
	}
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
