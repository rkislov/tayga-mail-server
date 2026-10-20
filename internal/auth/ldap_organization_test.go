package auth

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

type organizationTestLDAP struct {
	search func(*ldap.SearchRequest) (*ldap.SearchResult, error)
}

func (f organizationTestLDAP) Search(r *ldap.SearchRequest) (*ldap.SearchResult, error) {
	return f.search(r)
}

func organizationTestSource() (config.LDAPDomainConfig, storage.DirectorySource) {
	dc := config.LDAPDomainConfig{Enabled: true, BaseDN: "ou=People,dc=example,dc=com", OrganizationBaseDN: "ou=Groups,dc=example,dc=com"}
	s := storage.DefaultDirectorySource()
	s.Mode, s.BaseDN = "ldap", "ou=Departments,ou=Groups,dc=example,dc=com"
	return dc, s
}

func TestOrganizationSourceBoundaries(t *testing.T) {
	dc, source := organizationTestSource()
	d := NewDirectory(nil, config.LDAPConfig{Domains: map[string]config.LDAPDomainConfig{"example.com": dc}})
	if err := d.ValidateOrganizationSource("example.com", source); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"dc=example,dc=com", "ou=Groups,dc=other,dc=com", "", "bad-dn"} {
		bad := source
		bad.BaseDN = base
		if err := d.ValidateOrganizationSource("example.com", bad); !errors.Is(err, storage.ErrDirectoryInvalid) {
			t.Fatal(base, err)
		}
	}
	bad := source
	bad.Filter = "(invalid"
	if err := d.ValidateOrganizationSource("example.com", bad); err == nil {
		t.Fatal("invalid filter accepted")
	}
	if err := d.ValidateOrganizationSource("other.example", source); err == nil {
		t.Fatal("used another domain's LDAP")
	}
	source.Mode = "manual"
	if err := d.ValidateOrganizationSource("other.example", source); err != nil {
		t.Fatal("manual mode must not need LDAP", err)
	}
}

func TestOrganizationLDAPNestedGroupsAndExistingMailboxes(t *testing.T) {
	dc, source := organizationTestSource()
	rootDN, childDN := "cn=Company,"+source.BaseDN, "cn=Engineering,"+source.BaseDN
	personDN, missingDN := "cn=Person,"+dc.BaseDN, "cn=Unprovisioned,"+dc.BaseDN
	root := ldap.NewEntry(rootDN, map[string][]string{"CN": {"Компания"}, "MAIL": {"company@example.com"}, "entryUUID": {"root"}, "MEMBER": {strings.ToUpper(childDN), personDN}})
	child := ldap.NewEntry(childDN, map[string][]string{"cn": {"Разработка"}, "mail": {"engineering@example.com"}, "entryUUID": {"child"}, "member": {personDN, missingDN, personDN}})
	peopleCalls := 0
	conn := organizationTestLDAP{func(r *ldap.SearchRequest) (*ldap.SearchResult, error) {
		if r.BaseDN == source.BaseDN {
			if r.Scope != ldap.ScopeWholeSubtree || !r.EnforceSizeLimit || !strings.Contains(r.Filter, "(mail=*@example.com)") {
				t.Fatal(r)
			}
			return &ldap.SearchResult{Entries: []*ldap.Entry{root, child}}, nil
		}
		peopleCalls++
		email := "member@example.com"
		if r.BaseDN == missingDN {
			email = "external@other.example"
		}
		return &ldap.SearchResult{Entries: []*ldap.Entry{ldap.NewEntry(r.BaseDN, map[string][]string{"MAIL": {email}, "objectClass": {"person"}})}}, nil
	}}
	groups, report, err := readOrganization(context.Background(), conn, dc, "example.com", source, []*storage.User{{ID: "local", Email: "member@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[1].ParentExternalID != "uuid:root" || groups[1].Name != "Разработка" || !reflect.DeepEqual(groups[1].MemberIDs, []string{"local"}) {
		t.Fatal(groups)
	}
	if report.SkippedMembers != 1 || report.Memberships != 2 || peopleCalls != 2 || report.Groups[1].Parent != "Компания" {
		t.Fatal(report, peopleCalls)
	}
}

func TestOrganizationLDAPRejectsIncompleteAndAmbiguousTree(t *testing.T) {
	dc, source := organizationTestSource()
	child := "cn=Child," + source.BaseDN
	makeGroup := func(name string, members ...string) *ldap.Entry {
		return ldap.NewEntry("cn="+name+","+source.BaseDN, map[string][]string{"cn": {name}, "mail": {strings.ToLower(name) + "@example.com"}, "member": members})
	}
	for name, entries := range map[string][]*ldap.Entry{
		"multiple parents":  {makeGroup("A", child), makeGroup("B", child), makeGroup("Child")},
		"outside branch":    {ldap.NewEntry("cn=X,ou=Other,dc=example,dc=com", map[string][]string{"cn": {"X"}, "mail": {"x@example.com"}})},
		"missing member":    {makeGroup("A", "cn=Missing,"+dc.BaseDN)},
		"outside user base": {makeGroup("A", "cn=Other,dc=elsewhere")},
		"empty result":      {},
	} {
		t.Run(name, func(t *testing.T) {
			conn := organizationTestLDAP{func(r *ldap.SearchRequest) (*ldap.SearchResult, error) {
				if r.Scope == ldap.ScopeWholeSubtree {
					return &ldap.SearchResult{Entries: entries}, nil
				}
				return &ldap.SearchResult{}, nil
			}}
			if _, _, err := readOrganization(context.Background(), conn, dc, "example.com", source, nil); err == nil {
				t.Fatal("incomplete snapshot accepted")
			}
		})
	}
	for _, referrals := range [][]string{{"ldap://another.example"}, nil} {
		conn := organizationTestLDAP{func(*ldap.SearchRequest) (*ldap.SearchResult, error) {
			result := &ldap.SearchResult{Entries: []*ldap.Entry{makeGroup("A")}, Referrals: referrals}
			if referrals == nil {
				return result, ldap.NewError(ldap.LDAPResultSizeLimitExceeded, errors.New("partial"))
			}
			return result, nil
		}}
		if _, _, err := readOrganization(context.Background(), conn, dc, "example.com", source, nil); err == nil {
			t.Fatal("partial LDAP result accepted")
		}
	}
}

func TestOrganizationLDAPMemberRanges(t *testing.T) {
	entry := ldap.NewEntry("cn=Large,dc=example,dc=com", map[string][]string{"member;range=0-1": {"a", "b"}})
	conn := organizationTestLDAP{func(r *ldap.SearchRequest) (*ldap.SearchResult, error) {
		if !reflect.DeepEqual(r.Attributes, []string{"member;range=2-*"}) || r.Scope != ldap.ScopeBaseObject || r.BaseDN != entry.DN {
			t.Fatal(r)
		}
		return &ldap.SearchResult{Entries: []*ldap.Entry{ldap.NewEntry(entry.DN, map[string][]string{"member;range=2-*": {"c"}})}}, nil
	}}
	got, err := organizationGroupMembers(context.Background(), conn, entry, "member", 10)
	if err != nil || !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatal(got, err)
	}
	for _, attrs := range []map[string][]string{{}, {"member;range=3-*": {"d"}}, {"member;range=2-5": {"c"}}} {
		bad := organizationTestLDAP{func(*ldap.SearchRequest) (*ldap.SearchResult, error) {
			return &ldap.SearchResult{Entries: []*ldap.Entry{ldap.NewEntry(entry.DN, attrs)}}, nil
		}}
		if _, err := organizationGroupMembers(context.Background(), bad, entry, "member", 10); err == nil {
			t.Fatal("accepted partial AD members", attrs)
		}
	}
}

func TestOrganizationLDAPGroupPaging(t *testing.T) {
	calls := 0
	conn := organizationTestLDAP{func(r *ldap.SearchRequest) (*ldap.SearchResult, error) {
		calls++
		paging := ldap.FindControl(r.Controls, ldap.ControlTypePaging).(*ldap.ControlPaging)
		if calls == 2 && string(paging.Cookie) != "page2" {
			t.Fatal("cookie not forwarded")
		}
		next := ldap.NewControlPaging(200)
		if calls == 1 {
			next.SetCookie([]byte("page2"))
		}
		return &ldap.SearchResult{Entries: []*ldap.Entry{ldap.NewEntry("cn=Group", nil)}, Controls: []ldap.Control{next}}, nil
	}}
	result, err := organizationGroupSearch(context.Background(), conn, &ldap.SearchRequest{SizeLimit: 1001})
	if err != nil || len(result.Entries) != 2 || calls != 2 {
		t.Fatal(result, err, calls)
	}
	for _, count := range []int{0, 600} {
		calls = 0
		conn.search = func(*ldap.SearchRequest) (*ldap.SearchResult, error) {
			calls++
			next := ldap.NewControlPaging(200)
			next.SetCookie([]byte("repeated"))
			return &ldap.SearchResult{Entries: make([]*ldap.Entry, count), Controls: []ldap.Control{next}}, nil
		}
		if _, err := organizationGroupSearch(context.Background(), conn, &ldap.SearchRequest{}); err == nil || calls != 2 {
			t.Fatal("unbounded LDAP paging", err, calls)
		}
	}
}
