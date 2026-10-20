package auth

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

// OrganizationLDAPRoot is an administrator-controlled boundary, separate from
// the user search base when groups live in a sibling OU.
func (d *Directory) OrganizationLDAPRoot(domain string) (string, bool) {
	dc, ok := d.domainConfig(domain)
	if !ok || !dc.Enabled {
		return "", false
	}
	root := dc.OrganizationBaseDN
	if root == "" {
		root = dc.BaseDN
	}
	return root, true
}

func withinLDAPBranch(dn, root string) bool {
	child, err := ldap.ParseDN(dn)
	if err != nil {
		return false
	}
	parent, err := ldap.ParseDN(root)
	return err == nil && len(parent.RDNs) > 0 && (parent.EqualFold(child) || parent.AncestorOfFold(child))
}

func (d *Directory) ValidateOrganizationSource(domain string, source storage.DirectorySource) error {
	if source.Mode == "manual" {
		return nil
	}
	root, ok := d.OrganizationLDAPRoot(domain)
	if !ok {
		return fmt.Errorf("%w: LDAP authentication is not enabled for this domain", storage.ErrDirectoryInvalid)
	}
	if !withinLDAPBranch(source.BaseDN, root) {
		return fmt.Errorf("%w: organization branch must be inside the configured LDAP root", storage.ErrDirectoryInvalid)
	}
	if _, err := ldap.CompileFilter(source.Filter); err != nil {
		return fmt.Errorf("%w: invalid LDAP group filter", storage.ErrDirectoryInvalid)
	}
	return nil
}

type OrganizationPreviewGroup struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Parent      string `json:"parent"`
	MemberCount int    `json:"member_count"`
}

type OrganizationReport struct {
	Revision       int64                      `json:"revision"`
	Groups         []OrganizationPreviewGroup `json:"groups"`
	Memberships    int                        `json:"memberships"`
	SkippedMembers int                        `json:"skipped_members"`
	Warnings       []string                   `json:"warnings"`
}

type organizationLDAP interface {
	Search(*ldap.SearchRequest) (*ldap.SearchResult, error)
}

// Page explicitly so the cumulative group limit also bounds an LDAP server
// which ignores SizeLimit across pages. Partial pages are never published.
func organizationGroupSearch(ctx context.Context, conn organizationLDAP, request *ldap.SearchRequest) (*ldap.SearchResult, error) {
	page := ldap.NewControlPaging(200)
	request.Controls = []ldap.Control{page}
	request.EnforceSizeLimit = true
	all := &ldap.SearchResult{}
	cookies := map[string]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result, err := conn.Search(request)
		if err != nil {
			return nil, err
		}
		if result == nil || len(result.Referrals) != 0 {
			return nil, fmt.Errorf("%w: incomplete LDAP group search", storage.ErrDirectoryInvalid)
		}
		all.Entries = append(all.Entries, result.Entries...)
		if len(all.Entries) > 1000 {
			return nil, fmt.Errorf("%w: maximum 1000 LDAP groups", storage.ErrDirectoryInvalid)
		}
		control := ldap.FindControl(result.Controls, ldap.ControlTypePaging)
		if control == nil {
			return all, nil
		}
		next, ok := control.(*ldap.ControlPaging)
		if !ok {
			return nil, fmt.Errorf("%w: invalid LDAP paging control", storage.ErrDirectoryInvalid)
		}
		if len(next.Cookie) == 0 {
			return all, nil
		}
		if cookies[string(next.Cookie)] || len(cookies) >= 1000 {
			return nil, fmt.Errorf("%w: repeated LDAP paging cookie", storage.ErrDirectoryInvalid)
		}
		cookies[string(next.Cookie)] = true
		page.SetCookie(next.Cookie)
	}
}

func ldapDNKey(value string) (string, error) {
	dn, err := ldap.ParseDN(value)
	if err != nil || len(dn.RDNs) == 0 {
		return "", fmt.Errorf("%w: invalid LDAP DN", storage.ErrDirectoryInvalid)
	}
	return strings.ToLower(dn.String()), nil
}

// readOrganization never provisions accounts or grants roles. Only existing
// mailboxes in this domain can become group members.
func readOrganization(ctx context.Context, conn organizationLDAP, dc config.LDAPDomainConfig, domain string, source storage.DirectorySource, users []*storage.User) ([]storage.DirectoryImportGroup, *OrganizationReport, error) {
	report := &OrganizationReport{Revision: source.Revision, Groups: []OrganizationPreviewGroup{}, Warnings: []string{}}
	warn := func(message string) {
		report.SkippedMembers++
		if len(report.Warnings) < 20 {
			report.Warnings = append(report.Warnings, message)
		}
	}
	timeout := int(dc.Timeout.Seconds())
	if timeout < 1 {
		timeout = 10
	}
	filter := "(&" + source.Filter + "(" + source.EmailAttr + "=*@" + ldap.EscapeFilter(domain) + "))"
	result, err := organizationGroupSearch(ctx, conn, ldap.NewSearchRequest(source.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1001, timeout, false,
		filter, []string{source.NameAttr, source.EmailAttr, source.MemberAttr, "cn", "description", "entryUUID", "objectGUID"}, nil))
	if err != nil {
		return nil, report, fmt.Errorf("LDAP group search failed: %w", err)
	}
	if len(result.Referrals) != 0 || len(result.Entries) == 0 || len(result.Entries) > 1000 {
		return nil, report, fmt.Errorf("%w: LDAP must return a complete nonempty snapshot of at most 1000 groups", storage.ErrDirectoryInvalid)
	}
	byDN := map[string]int{}
	groups := []storage.DirectoryImportGroup{}
	for _, entry := range result.Entries {
		if !withinLDAPBranch(entry.DN, source.BaseDN) {
			return nil, report, fmt.Errorf("%w: group outside selected branch", storage.ErrDirectoryInvalid)
		}
		key, err := ldapDNKey(entry.DN)
		if err != nil {
			return nil, report, err
		}
		if _, exists := byDN[key]; exists {
			return nil, report, fmt.Errorf("%w: duplicate LDAP group", storage.ErrDirectoryInvalid)
		}
		email := strings.ToLower(strings.TrimSpace(entry.GetEqualFoldAttributeValue(source.EmailAttr)))
		local, suffix, ok := strings.Cut(email, "@")
		if !ok || suffix != domain {
			return nil, report, fmt.Errorf("%w: group address must belong to this domain", storage.ErrDirectoryInvalid)
		}
		id := "dn:" + key
		if guid := entry.GetEqualFoldRawAttributeValue("objectGUID"); len(guid) > 0 {
			id = "guid:" + hex.EncodeToString(guid)
		} else if uuid := entry.GetEqualFoldAttributeValue("entryUUID"); uuid != "" {
			id = "uuid:" + strings.ToLower(uuid)
		}
		name := entry.GetEqualFoldAttributeValue(source.NameAttr)
		if name == "" {
			name = entry.GetEqualFoldAttributeValue("cn")
		}
		byDN[key] = len(groups)
		groups = append(groups, storage.DirectoryImportGroup{ExternalID: id, LocalPart: local, Name: name, Description: entry.GetEqualFoldAttributeValue("description"), MemberIDs: []string{}})
	}
	localUsers := map[string]*storage.User{}
	for _, user := range users {
		if user.AuthSource != "resource" {
			localUsers[strings.ToLower(user.Email)] = user
		}
	}
	resolved := map[string]string{}
	emailAttr := dc.AttrEmail
	if emailAttr == "" {
		emailAttr = "mail"
	}
	totalMembers := 0
	for i, entry := range result.Entries {
		dns, err := organizationGroupMembers(ctx, conn, entry, source.MemberAttr, timeout)
		if err != nil {
			return nil, report, err
		}
		totalMembers += len(dns)
		if totalMembers > 100000 {
			return nil, report, fmt.Errorf("%w: too many LDAP memberships", storage.ErrDirectoryInvalid)
		}
		seenMembers := map[string]bool{}
		for _, memberDN := range dns {
			if err := ctx.Err(); err != nil {
				return nil, report, err
			}
			key, err := ldapDNKey(memberDN)
			if err != nil {
				return nil, report, err
			}
			if child, found := byDN[key]; found {
				if groups[child].ParentExternalID != "" && groups[child].ParentExternalID != groups[i].ExternalID {
					return nil, report, fmt.Errorf("%w: a nested group has several parent departments", storage.ErrDirectoryInvalid)
				}
				groups[child].ParentExternalID = groups[i].ExternalID
				continue
			}
			id, known := resolved[key]
			if !known {
				if len(resolved) >= 10000 {
					return nil, report, fmt.Errorf("%w: too many LDAP members", storage.ErrDirectoryInvalid)
				}
				if !withinLDAPBranch(memberDN, dc.BaseDN) {
					return nil, report, fmt.Errorf("%w: member outside the configured user branch; include nested groups in the selected group branch", storage.ErrDirectoryInvalid)
				}
				person, err := conn.Search(ldap.NewSearchRequest(memberDN, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, timeout, false,
					"(objectClass=*)", []string{emailAttr, "objectClass"}, nil))
				if err != nil || len(person.Entries) != 1 || len(person.Referrals) != 0 {
					return nil, report, fmt.Errorf("LDAP member lookup failed; previous directory retained")
				}
				for _, class := range person.Entries[0].GetEqualFoldAttributeValues("objectClass") {
					if strings.EqualFold(class, "group") || strings.EqualFold(class, "groupOfNames") || strings.EqualFold(class, "groupOfUniqueNames") {
						return nil, report, fmt.Errorf("%w: nested group is outside the selected group branch or filter", storage.ErrDirectoryInvalid)
					}
				}
				email := strings.ToLower(strings.TrimSpace(person.Entries[0].GetEqualFoldAttributeValue(emailAttr)))
				if user := localUsers[email]; user != nil {
					id = user.ID
				} else {
					warn("No mailbox in this domain for LDAP member: " + email)
				}
				resolved[key] = id
			}
			if id != "" && !seenMembers[id] {
				seenMembers[id] = true
				groups[i].MemberIDs = append(groups[i].MemberIDs, id)
				report.Memberships++
			}
		}
	}
	names := map[string]string{}
	for _, group := range groups {
		names[group.ExternalID] = group.Name
	}
	for _, group := range groups {
		report.Groups = append(report.Groups, OrganizationPreviewGroup{Name: group.Name, Email: group.LocalPart + "@" + domain, Parent: names[group.ParentExternalID], MemberCount: len(group.MemberIDs)})
	}
	return groups, report, nil
}

// AD returns large groups as member;range=0-1499, etc. Never apply a partial
// membership list: fetch every range, validate continuity and cap the total.
func organizationGroupMembers(ctx context.Context, conn organizationLDAP, entry *ldap.Entry, attr string, timeout int) ([]string, error) {
	var members []string
	start := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		found := false
		for _, a := range entry.Attributes {
			if strings.EqualFold(a.Name, attr) {
				if start != 0 || len(a.Values) > 10000 {
					return nil, fmt.Errorf("%w: invalid LDAP member range", storage.ErrDirectoryInvalid)
				}
				return a.Values, nil
			}
			prefix := strings.ToLower(attr) + ";range="
			if !strings.HasPrefix(strings.ToLower(a.Name), prefix) {
				continue
			}
			found = true
			bounds := strings.Split(strings.ToLower(a.Name)[len(prefix):], "-")
			if len(bounds) != 2 {
				return nil, fmt.Errorf("%w: malformed LDAP member range", storage.ErrDirectoryInvalid)
			}
			from, err := strconv.Atoi(bounds[0])
			if err != nil || from != start {
				return nil, fmt.Errorf("%w: incomplete LDAP member range", storage.ErrDirectoryInvalid)
			}
			members = append(members, a.Values...)
			if len(members) > 10000 {
				return nil, fmt.Errorf("%w: too many group members", storage.ErrDirectoryInvalid)
			}
			if bounds[1] == "*" {
				return members, nil
			}
			end, err := strconv.Atoi(bounds[1])
			if err != nil || end < start || end-start+1 != len(a.Values) {
				return nil, fmt.Errorf("%w: incomplete LDAP member range", storage.ErrDirectoryInvalid)
			}
			start = end + 1
			break
		}
		if !found {
			if start == 0 {
				return []string{}, nil
			}
			return nil, fmt.Errorf("%w: missing LDAP member range", storage.ErrDirectoryInvalid)
		}
		result, err := conn.Search(ldap.NewSearchRequest(entry.DN, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, timeout, false,
			"(objectClass=*)", []string{fmt.Sprintf("%s;range=%d-*", attr, start)}, nil))
		if err != nil || len(result.Entries) != 1 || len(result.Referrals) != 0 {
			return nil, fmt.Errorf("LDAP member range lookup failed")
		}
		entry = result.Entries[0]
	}
}

// LoadOrganization fetches a complete source snapshot with a bounded lifetime.
// Preview is read-only; apply commits the same snapshot atomically.
func (d *Directory) LoadOrganization(ctx context.Context, domainID string, apply bool) (*OrganizationReport, error) {
	domain, err := d.Store.GetDomainByID(ctx, domainID)
	if err != nil {
		return nil, err
	}
	source, err := d.Store.GetDirectorySource(ctx, domainID)
	if err != nil {
		return nil, err
	}
	if source.Mode != "ldap" {
		return nil, fmt.Errorf("%w: select LDAP source first", storage.ErrDirectoryInvalid)
	}
	// Persist only a safe error summary, never bind errors or credentials.
	if apply {
		defer func() {
			if err != nil {
				recordCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = d.Store.RecordDirectoryImportError(recordCtx, domainID, source.Revision, "LDAP synchronization failed; previous directory retained")
			}
		}()
	}
	if err = d.ValidateOrganizationSource(domain.Name, *source); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	dc, _ := d.domainConfig(domain.Name)
	conn, err := d.dial(dc)
	if err != nil {
		return nil, fmt.Errorf("LDAP connection failed")
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	conn.SetTimeout(10 * time.Second)
	if dc.BindDN != "" {
		if err = conn.Bind(dc.BindDN, dc.BindPassword); err != nil {
			return nil, fmt.Errorf("LDAP service account bind failed")
		}
	}
	users, err := d.Store.ListUsersByDomain(ctx, domainID)
	if err != nil {
		return nil, err
	}
	groups, report, err := readOrganization(ctx, conn, dc, domain.Name, *source, users)
	if err != nil {
		return report, err
	}
	if apply {
		var saved *storage.DirectorySource
		saved, err = d.Store.ApplyDirectoryImport(ctx, domainID, source.Revision, groups)
		if err == nil {
			report.Revision = saved.Revision
		}
	} else {
		err = d.Store.ValidateDirectoryImport(ctx, domainID, source.Revision, groups)
	}
	return report, err
}

func (d *Directory) StartOrganizationSyncLoop(ctx context.Context, log *slog.Logger) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for name, dc := range d.Cfg.Domains {
					if !dc.Enabled || ctx.Err() != nil {
						continue
					}
					domain, err := d.Store.GetDomainByName(ctx, name)
					if err != nil {
						continue
					}
					source, err := d.Store.GetDirectorySource(ctx, domain.ID)
					if err != nil || source.Mode != "ldap" || source.IntervalMinutes < 5 || time.Since(source.LastAttempt) < time.Duration(source.IntervalMinutes)*time.Minute {
						continue
					}
					report, err := d.LoadOrganization(ctx, domain.ID, true)
					if log != nil {
						if err != nil {
							log.Warn("LDAP organization sync failed; previous directory retained", "domain", name, "error", err)
						} else {
							log.Info("LDAP organization synchronized", "domain", name, "groups", len(report.Groups), "memberships", report.Memberships)
						}
					}
				}
			}
		}
	}()
}
