package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

// Directory implements LDAPProvider with per-domain configs and JIT provisioning.
type Directory struct {
	Store storage.Driver
	Cfg   config.LDAPConfig
	cache *authCache
}

func NewDirectory(store storage.Driver, cfg config.LDAPConfig) *Directory {
	return &Directory{
		Store: store,
		Cfg:   cfg,
		cache: newAuthCache(cfg.CacheTTL),
	}
}

// EnabledFor reports whether LDAP is configured for the domain.
func (d *Directory) EnabledFor(domain string) bool {
	if d == nil {
		return false
	}
	dc, ok := d.domainConfig(domain)
	return ok && dc.Enabled
}

func (d *Directory) domainConfig(domain string) (config.LDAPDomainConfig, bool) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if d.Cfg.Domains == nil {
		return config.LDAPDomainConfig{}, false
	}
	dc, ok := d.Cfg.Domains[domain]
	return dc, ok
}

// Authenticate verifies credentials against the domain LDAP and ensures a local user row.
func (d *Directory) Authenticate(ctx context.Context, domain, username, password string) (*storage.User, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	domain = strings.ToLower(strings.TrimSpace(domain))
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	if domain == "" {
		if at := strings.LastIndex(username, "@"); at >= 0 {
			domain = username[at+1:]
		}
	}

	dc, ok := d.domainConfig(domain)
	if !ok || !dc.Enabled {
		return nil, ErrUnsupportedSource
	}

	if userID, okAuth, hit := d.cache.get(username, password); hit {
		if !okAuth {
			return nil, ErrInvalidCredentials
		}
		return d.Store.GetUserByID(ctx, userID)
	}

	entry, err := d.searchAndBind(ctx, dc, username, password)
	if err != nil {
		d.cache.setFail(username, password)
		return nil, err
	}

	u, err := d.ensureUser(ctx, domain, username, entry, dc)
	if err != nil {
		return nil, err
	}

	groups := entry.Groups
	mode := strings.ToLower(strings.TrimSpace(dc.Groups.Mode))
	if mode == "search" || (mode == "memberof" && len(groups) == 0) {
		if g, gerr := d.resolveGroups(ctx, dc, entry); gerr == nil {
			groups = g
		}
	}
	u, err = d.applyRoles(ctx, u, dc, groups)
	if err != nil {
		return nil, err
	}

	d.cache.setOK(username, password, u.ID)
	return u, nil
}

type ldapPerson struct {
	DN          string
	Email       string
	DisplayName string
	Groups      []string
}

func (d *Directory) searchAndBind(ctx context.Context, dc config.LDAPDomainConfig, username, password string) (*ldapPerson, error) {
	conn, err := d.dial(dc)
	if err != nil {
		return nil, fmt.Errorf("ldap dial: %w", err)
	}
	defer conn.Close()

	conn.SetTimeout(dc.Timeout)
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetTimeout(time.Until(deadline))
	}

	if dc.BindDN != "" {
		if err := conn.Bind(dc.BindDN, dc.BindPassword); err != nil {
			return nil, ErrInvalidCredentials
		}
	}

	attrs := []string{"dn", dc.AttrEmail, dc.AttrName}
	if strings.EqualFold(dc.Groups.Mode, "memberof") {
		attr := dc.Groups.AttrMemberOf
		if attr == "" {
			attr = "memberOf"
		}
		attrs = append(attrs, attr)
	}

	filter := ExpandFilter(dc.Filter, username)
	req := ldap.NewSearchRequest(
		dc.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, int(dc.Timeout.Seconds()), false,
		filter,
		attrs,
		nil,
	)
	res, err := conn.Search(req)
	if err != nil || len(res.Entries) == 0 {
		return nil, ErrInvalidCredentials
	}
	entry := res.Entries[0]

	// User bind to verify password.
	if err := conn.Bind(entry.DN, password); err != nil {
		return nil, ErrInvalidCredentials
	}

	email := strings.ToLower(entry.GetAttributeValue(dc.AttrEmail))
	if email == "" {
		email = username
	}
	name := entry.GetAttributeValue(dc.AttrName)
	person := &ldapPerson{DN: entry.DN, Email: email, DisplayName: name}
	if strings.EqualFold(dc.Groups.Mode, "memberof") {
		attr := dc.Groups.AttrMemberOf
		if attr == "" {
			attr = "memberOf"
		}
		for _, v := range entry.GetAttributeValues(attr) {
			v = strings.TrimSpace(v)
			if v != "" {
				person.Groups = append(person.Groups, v)
			}
		}
	}
	return person, nil
}

func (d *Directory) dial(dc config.LDAPDomainConfig) (*ldap.Conn, error) {
	u, err := url.Parse(dc.URL)
	if err != nil {
		return nil, err
	}
	addr := u.Host
	if addr == "" {
		addr = dc.URL
	}

	var conn *ldap.Conn
	switch strings.ToLower(u.Scheme) {
	case "ldaps":
		conn, err = ldap.DialURL(dc.URL, ldap.DialWithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	default:
		conn, err = ldap.DialURL(dc.URL)
	}
	if err != nil {
		return nil, err
	}
	if dc.StartTLS && strings.ToLower(u.Scheme) != "ldaps" {
		if err := conn.StartTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: u.Hostname()}); err != nil {
			conn.Close()
			return nil, err
		}
	}
	_ = addr
	return conn, nil
}

func (d *Directory) ensureUser(ctx context.Context, domain, username string, person *ldapPerson, dc config.LDAPDomainConfig) (*storage.User, error) {
	email := person.Email
	if email == "" {
		email = username
	}
	email = strings.ToLower(email)

	if existing, err := d.Store.GetUserByEmail(ctx, email); err == nil {
		if person.DisplayName != "" && person.DisplayName != existing.DisplayName {
			_ = d.Store.UpdateUserProfile(ctx, existing.ID, person.DisplayName)
			existing.DisplayName = person.DisplayName
		}
		if existing.AuthSource != "ldap" {
			// Do not hijack local accounts via LDAP email match.
			if existing.AuthSource == "" || existing.AuthSource == "local" {
				return nil, ErrInvalidCredentials
			}
		}
		if !existing.Enabled {
			return nil, ErrUserDisabled
		}
		return existing, nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}

	dom, err := d.Store.GetDomainByName(ctx, domain)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, err
		}
		// Auto-create tenant/domain for JIT if missing.
		tenant, terr := d.Store.GetTenantByName(ctx, "default")
		if errors.Is(terr, storage.ErrNotFound) {
			tenant, terr = d.Store.CreateTenant(ctx, "default")
		}
		if terr != nil {
			return nil, terr
		}
		dom, err = d.Store.CreateDomain(ctx, tenant.ID, domain)
		if err != nil {
			return nil, err
		}
	}

	local := email
	if at := strings.LastIndex(email, "@"); at >= 0 {
		local = email[:at]
	}
	name := person.DisplayName
	if name == "" {
		name = local
	}
	u, err := d.Store.CreateUser(ctx, &storage.User{
		TenantID:     dom.TenantID,
		DomainID:     dom.ID,
		Email:        email,
		LocalPart:    local,
		DisplayName:  name,
		PasswordHash: "",
		AuthSource:   "ldap",
		Enabled:      true,
	})
	if err != nil {
		return nil, err
	}
	_ = dc
	return u, nil
}

// ExpandFilter replaces {email} and {user} placeholders in an LDAP filter.
func ExpandFilter(filter, email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	user := email
	if at := strings.LastIndex(email, "@"); at >= 0 {
		user = email[:at]
	}
	// Escape LDAP filter special chars in substitutions.
	escEmail := ldap.EscapeFilter(email)
	escUser := ldap.EscapeFilter(user)
	out := strings.ReplaceAll(filter, "{email}", escEmail)
	out = strings.ReplaceAll(out, "{user}", escUser)
	return out
}
