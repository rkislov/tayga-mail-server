package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

// SyncReport summarizes a domain group sync run.
type SyncReport struct {
	Domain  string `json:"domain"`
	Created int    `json:"created"`
	Updated int    `json:"updated"`
	Admins  int    `json:"admins"`
	Errors  int    `json:"errors"`
	Skipped int    `json:"skipped"`
	Message string `json:"message,omitempty"`
}

// SyncDomain provisions users from configured sync.group_dns / admin_groups and refreshes roles.
func (d *Directory) SyncDomain(ctx context.Context, domain string) (*SyncReport, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	rep := &SyncReport{Domain: domain}
	dc, ok := d.domainConfig(domain)
	if !ok || !dc.Enabled {
		rep.Message = "domain not enabled"
		return rep, fmt.Errorf("ldap domain %q not enabled", domain)
	}
	if !dc.Sync.Enabled && len(dc.Sync.GroupDNs) == 0 && len(dc.Groups.AdminGroups) == 0 {
		rep.Message = "no sync/groups configured"
		return rep, nil
	}

	conn, err := d.dial(dc)
	if err != nil {
		return rep, err
	}
	defer conn.Close()
	conn.SetTimeout(dc.Timeout)
	if dc.BindDN != "" {
		if err := conn.Bind(dc.BindDN, dc.BindPassword); err != nil {
			return rep, fmt.Errorf("ldap bind: %w", err)
		}
	}

	memberAttr := dc.Sync.MemberAttr
	if memberAttr == "" {
		memberAttr = "member"
	}

	type memberInfo struct {
		person *ldapPerson
		groups []string
	}
	byEmail := map[string]*memberInfo{}

	var groupDNs []string
	groupDNs = append(groupDNs, dc.Sync.GroupDNs...)
	groupDNs = append(groupDNs, dc.Groups.AdminGroups...)

	for _, gdn := range groupDNs {
		gdn = strings.TrimSpace(gdn)
		if gdn == "" || !strings.Contains(gdn, "=") {
			rep.Skipped++
			continue
		}
		members, err := d.groupMembers(conn, dc, gdn, memberAttr)
		if err != nil {
			rep.Errors++
			continue
		}
		for _, mdn := range members {
			person, err := d.readPerson(conn, dc, mdn)
			if err != nil || person.Email == "" {
				rep.Errors++
				continue
			}
			email := strings.ToLower(person.Email)
			info := byEmail[email]
			if info == nil {
				info = &memberInfo{person: person}
				byEmail[email] = info
			}
			info.groups = append(info.groups, gdn)
			if cn := groupCN(gdn); cn != "" {
				info.groups = append(info.groups, cn)
			}
		}
	}

	for _, info := range byEmail {
		existing, err := d.Store.GetUserByEmail(ctx, info.person.Email)
		if err != nil {
			if err != storage.ErrNotFound {
				rep.Errors++
				continue
			}
			u, cerr := d.ensureUser(ctx, domain, info.person.Email, info.person, dc)
			if cerr != nil {
				rep.Errors++
				continue
			}
			existing = u
			rep.Created++
		} else {
			rep.Updated++
		}
		groups := info.groups
		if g, gerr := d.resolveGroups(ctx, dc, info.person); gerr == nil && len(g) > 0 {
			groups = g
		}
		u, aerr := d.applyRoles(ctx, existing, dc, groups)
		if aerr != nil {
			rep.Errors++
			continue
		}
		if storage.HasRole(u.Roles, storage.RoleAdmin) {
			rep.Admins++
		}
	}
	return rep, nil
}

func (d *Directory) groupMembers(conn *ldap.Conn, dc config.LDAPDomainConfig, groupDN, memberAttr string) ([]string, error) {
	req := ldap.NewSearchRequest(
		groupDN,
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, int(dc.Timeout.Seconds()), false,
		"(objectClass=*)",
		[]string{memberAttr},
		nil,
	)
	res, err := conn.Search(req)
	if err != nil || len(res.Entries) == 0 {
		return nil, err
	}
	return res.Entries[0].GetAttributeValues(memberAttr), nil
}

func (d *Directory) readPerson(conn *ldap.Conn, dc config.LDAPDomainConfig, dn string) (*ldapPerson, error) {
	req := ldap.NewSearchRequest(
		dn,
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, int(dc.Timeout.Seconds()), false,
		"(objectClass=*)",
		[]string{dc.AttrEmail, dc.AttrName},
		nil,
	)
	res, err := conn.Search(req)
	if err != nil || len(res.Entries) == 0 {
		return nil, fmt.Errorf("read %s: %w", dn, err)
	}
	e := res.Entries[0]
	email := strings.ToLower(e.GetAttributeValue(dc.AttrEmail))
	return &ldapPerson{DN: dn, Email: email, DisplayName: e.GetAttributeValue(dc.AttrName)}, nil
}

// SyncAll runs SyncDomain for every enabled LDAP domain with sync/groups configured.
func (d *Directory) SyncAll(ctx context.Context) ([]*SyncReport, error) {
	var out []*SyncReport
	for name, dc := range d.Cfg.Domains {
		if !dc.Enabled {
			continue
		}
		if !dc.Sync.Enabled && len(dc.Sync.GroupDNs) == 0 && len(dc.Groups.AdminGroups) == 0 {
			continue
		}
		rep, err := d.SyncDomain(ctx, name)
		if rep != nil {
			out = append(out, rep)
		}
		if err != nil && rep == nil {
			return out, err
		}
	}
	return out, nil
}

// StartGroupSyncLoop runs periodic LDAP group sync when sync.enabled and interval > 0.
func (d *Directory) StartGroupSyncLoop(ctx context.Context, log *slog.Logger) {
	if d == nil {
		return
	}
	for name, dc := range d.Cfg.Domains {
		if !dc.Enabled || !dc.Sync.Enabled || dc.Sync.Interval <= 0 {
			continue
		}
		domain := name
		interval := dc.Sync.Interval
		go func() {
			t := time.NewTicker(interval)
			defer t.Stop()
			if log != nil {
				log.Info("ldap group sync loop", "domain", domain, "interval", interval.String())
			}
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					rep, err := d.SyncDomain(ctx, domain)
					if log == nil {
						continue
					}
					if err != nil {
						log.Warn("ldap sync", "domain", domain, "err", err)
						continue
					}
					log.Info("ldap sync", "domain", domain, "created", rep.Created, "updated", rep.Updated, "admins", rep.Admins, "errors", rep.Errors)
				}
			}
		}()
	}
}
