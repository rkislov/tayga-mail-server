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

	var direct []string
	switch mode {
	case "memberof":
		// Prefer groups already read during user search (memberOf attribute).
		if len(person.Groups) > 0 {
			direct = append(direct, person.Groups...)
		} else {
			var gerr error
			direct, gerr = d.groupsMemberOf(conn, dc, person.DN)
			if gerr != nil {
				return nil, gerr
			}
		}
	case "search":
		var gerr error
		direct, gerr = d.groupsSearch(conn, dc, person.DN)
		if gerr != nil {
			return nil, gerr
		}
	default:
		return nil, nil
	}
	if !dc.Groups.Nested {
		return direct, nil
	}
	return d.expandNestedGroups(conn, dc, person.DN, direct)
}

// expandNestedGroups adds transitive parent groups (nested membership).
func (d *Directory) expandNestedGroups(conn *ldap.Conn, dc config.LDAPDomainConfig, userDN string, direct []string) ([]string, error) {
	mode := strings.ToLower(strings.TrimSpace(dc.Groups.NestedMode))
	if mode == "" {
		mode = "walk"
	}
	switch mode {
	case "chain":
		return d.groupsChain(conn, dc, userDN)
	default:
		return expandGroupClosure(direct, func(dn string) ([]string, error) {
			return d.parentGroups(conn, dc, dn)
		}, dc.Groups.MaxDepth), nil
	}
}

// groupsChain uses Active Directory LDAP_MATCHING_RULE_IN_CHAIN (1.2.840.113556.1.4.1941).
func (d *Directory) groupsChain(conn *ldap.Conn, dc config.LDAPDomainConfig, userDN string) ([]string, error) {
	base := dc.Groups.BaseDN
	if base == "" {
		base = dc.BaseDN
	}
	attrName := dc.Groups.AttrName
	if attrName == "" {
		attrName = "cn"
	}
	memberAttr := dc.Groups.MemberAttr
	if memberAttr == "" {
		memberAttr = "member"
	}
	filter := fmt.Sprintf("(%s:1.2.840.113556.1.4.1941:=%s)", memberAttr, ldap.EscapeFilter(userDN))
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

func (d *Directory) parentGroups(conn *ldap.Conn, dc config.LDAPDomainConfig, groupDN string) ([]string, error) {
	groupDN = strings.TrimSpace(groupDN)
	if groupDN == "" || !strings.Contains(groupDN, "=") {
		return nil, nil
	}
	// Prefer memberOf on the group entry (AD / memberof overlay).
	attr := dc.Groups.AttrMemberOf
	if attr == "" {
		attr = "memberOf"
	}
	req := ldap.NewSearchRequest(
		groupDN,
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, int(dc.Timeout.Seconds()), false,
		"(objectClass=*)",
		[]string{attr},
		nil,
	)
	res, err := conn.Search(req)
	if err == nil && len(res.Entries) > 0 {
		vals := res.Entries[0].GetAttributeValues(attr)
		if len(vals) > 0 {
			var out []string
			for _, v := range vals {
				v = strings.TrimSpace(v)
				if v != "" {
					out = append(out, v)
				}
			}
			return out, nil
		}
	}
	// Fallback: search for groups that list this DN as member.
	base := dc.Groups.BaseDN
	if base == "" {
		base = dc.BaseDN
	}
	memberAttr := dc.Groups.MemberAttr
	if memberAttr == "" {
		memberAttr = "member"
	}
	attrName := dc.Groups.AttrName
	if attrName == "" {
		attrName = "cn"
	}
	filter := fmt.Sprintf("(%s=%s)", memberAttr, ldap.EscapeFilter(groupDN))
	sreq := ldap.NewSearchRequest(
		base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, int(dc.Timeout.Seconds()), false,
		filter,
		[]string{"dn", attrName},
		nil,
	)
	sres, err := conn.Search(sreq)
	if err != nil {
		return nil, nil // best-effort
	}
	var out []string
	for _, e := range sres.Entries {
		out = append(out, e.DN)
	}
	return out, nil
}

// expandGroupClosure BFS-expands seed group DNs via parentsOf, up to maxDepth.
// Non-DN tokens (plain CNs) are kept in the result but not walked.
func expandGroupClosure(seed []string, parentsOf func(dn string) ([]string, error), maxDepth int) []string {
	if maxDepth <= 0 {
		maxDepth = 8
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(g string) {
		g = strings.TrimSpace(g)
		if g == "" {
			return
		}
		key := strings.ToLower(g)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, g)
		if cn := groupCN(g); cn != "" && cn != key {
			if _, ok := seen[cn]; !ok {
				seen[cn] = struct{}{}
				out = append(out, cn)
			}
		}
	}
	var frontier []string
	for _, g := range seed {
		add(g)
		if strings.Contains(g, "=") {
			frontier = append(frontier, g)
		}
	}
	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		var next []string
		for _, dn := range frontier {
			parents, err := parentsOf(dn)
			if err != nil {
				continue
			}
			for _, p := range parents {
				key := strings.ToLower(strings.TrimSpace(p))
				if key == "" {
					continue
				}
				if _, ok := seen[key]; ok {
					continue
				}
				add(p)
				if strings.Contains(p, "=") {
					next = append(next, p)
				}
			}
		}
		frontier = next
	}
	return out
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
