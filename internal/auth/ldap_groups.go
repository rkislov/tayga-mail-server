package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

// resolveGroups returns LDAP group DNs/CNs for the authenticated person.
func (d *Directory) resolveGroups(ctx context.Context, dc config.LDAPDomainConfig, person *ldapPerson) ([]string, error) {
	mode := strings.ToLower(strings.TrimSpace(dc.Groups.Mode))
	if mode == "" || mode == "off" || mode == "none" {
		return nil, nil
	}
	conn, err := d.dial(dc)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetTimeout(dc.Timeout)
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetTimeout(time.Until(deadline))
	}
	if dc.BindDN != "" {
		if err := conn.Bind(dc.BindDN, dc.BindPassword); err != nil {
			return nil, fmt.Errorf("ldap group bind: %w", err)
		}
	}

	switch mode {
	case "memberof":
		return d.groupsMemberOf(conn, dc, person.DN)
	case "search":
		return d.groupsSearch(conn, dc, person.DN)
	default:
		return nil, nil
	}
}

func (d *Directory) groupsMemberOf(conn *ldap.Conn, dc config.LDAPDomainConfig, userDN string) ([]string, error) {
	attr := dc.Groups.AttrMemberOf
	if attr == "" {
		attr = "memberOf"
	}
	req := ldap.NewSearchRequest(
		userDN,
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, int(dc.Timeout.Seconds()), false,
		"(objectClass=*)",
		[]string{attr},
		nil,
	)
	res, err := conn.Search(req)
	if err != nil || len(res.Entries) == 0 {
		return nil, err
	}
	vals := res.Entries[0].GetAttributeValues(attr)
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out, nil
}

func (d *Directory) groupsSearch(conn *ldap.Conn, dc config.LDAPDomainConfig, userDN string) ([]string, error) {
	base := dc.Groups.BaseDN
	if base == "" {
		base = dc.BaseDN
	}
	filter := dc.Groups.Filter
	if filter == "" {
		filter = "(&(objectClass=groupOfNames)(member={dn}))"
	}
	filter = strings.ReplaceAll(filter, "{dn}", ldap.EscapeFilter(userDN))
	attrName := dc.Groups.AttrName
	if attrName == "" {
		attrName = "cn"
	}
	req := ldap.NewSearchRequest(
		base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, int(dc.Timeout.Seconds()), false,
		filter,
		[]string{"dn", attrName},
		nil,
	)
	res, err := conn.Search(req)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range res.Entries {
		out = append(out, e.DN)
		if cn := e.GetAttributeValue(attrName); cn != "" {
			out = append(out, cn)
		}
	}
	return out, nil
}

func groupMatches(userGroups, adminGroups []string) bool {
	normUser := map[string]struct{}{}
	for _, g := range userGroups {
		g = strings.ToLower(strings.TrimSpace(g))
		if g == "" {
			continue
		}
		normUser[g] = struct{}{}
		if cn := groupCN(g); cn != "" {
			normUser[cn] = struct{}{}
		}
	}
	for _, want := range adminGroups {
		want = strings.ToLower(strings.TrimSpace(want))
		if want == "" {
			continue
		}
		if _, ok := normUser[want]; ok {
			return true
		}
		if cn := groupCN(want); cn != "" {
			if _, ok := normUser[cn]; ok {
				return true
			}
		}
	}
	return false
}

func groupCN(dnOrCN string) string {
	dnOrCN = strings.TrimSpace(dnOrCN)
	if !strings.Contains(dnOrCN, "=") {
		return strings.ToLower(dnOrCN)
	}
	// cn=mail-admins,ou=groups,...
	parts := strings.Split(dnOrCN, ",")
	if len(parts) == 0 {
		return ""
	}
	kv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
	if len(kv) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(kv[1]))
}

func (d *Directory) applyRoles(ctx context.Context, u *storage.User, dc config.LDAPDomainConfig, groups []string) (*storage.User, error) {
	if len(dc.Groups.AdminGroups) == 0 {
		return u, nil
	}
	wantAdmin := groupMatches(groups, dc.Groups.AdminGroups)
	roles := u.Roles
	if wantAdmin {
		roles = storage.WithRole(roles, storage.RoleAdmin)
	} else {
		// LDAP admin_groups are the source of truth for the admin role.
		roles = storage.WithoutRole(roles, storage.RoleAdmin)
	}
	roles = storage.JoinRoles(storage.ParseRoles(roles))
	if roles == storage.JoinRoles(storage.ParseRoles(u.Roles)) {
		return u, nil
	}
	if err := d.Store.UpdateUserRoles(ctx, u.ID, roles); err != nil {
		return nil, err
	}
	u.Roles = roles
	return u, nil
}
