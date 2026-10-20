package auth

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ber "github.com/go-asn1-ber/asn1-ber"
	ldap "github.com/go-ldap/ldap/v3"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

// Serve real LDAP Bind/Search packets over TCP to exercise connection, decoding,
// preview, publication and failure recording together, without an external AD.
func TestOrganizationLDAPLoadOverTCP(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "ldap.db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tenant, _ := store.CreateTenant(ctx, "Example")
	domain, _ := store.CreateDomain(ctx, tenant.ID, "example.com")
	user, err := store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: "person", Email: "person@example.com", AuthSource: "local", Enabled: true, Roles: "user"})
	if err != nil {
		t.Fatal(err)
	}
	dc, source := organizationTestSource()
	current, err := store.SaveDirectorySource(ctx, domain.ID, source)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var partial atomic.Bool
	var wg sync.WaitGroup
	defer func() { listener.Close(); wg.Wait() }()
	errs := make(chan error, 20)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(3 * time.Second))
				for {
					packet, err := ber.ReadPacket(conn)
					if err != nil {
						return
					}
					id := packet.Children[0].Value.(int64)
					req := packet.Children[1]
					response := func(operation *ber.Packet) {
						msg := ber.NewSequence("LDAPMessage")
						msg.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, id, "id"))
						msg.AppendChild(operation)
						if _, err := conn.Write(msg.Bytes()); err != nil {
							errs <- err
						}
					}
					done := func(tag ber.Tag, code int64) {
						op := ber.Encode(ber.ClassApplication, ber.TypeConstructed, tag, nil, "")
						op.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, code, "result"))
						op.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "matchedDN"))
						op.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "diagnostic"))
						response(op)
					}
					switch req.Tag {
					case 0: // Bind
						if req.Children[1].Value != "cn=reader,dc=example,dc=com" || req.Children[2].Data.String() != "test-password" {
							errs <- errors.New("wrong service bind")
						}
						done(1, 0)
					case 3: // Search
						base := req.Children[0].Value.(string)
						entry := ldap.NewEntry("cn=Team,"+source.BaseDN, map[string][]string{
							"cn": {"Разработка"}, "mail": {"team@example.com"}, "entryUUID": {"stable-team"},
							"member": {"cn=Person," + dc.BaseDN},
						})
						if base != source.BaseDN {
							entry = ldap.NewEntry(base, map[string][]string{"mail": {user.Email}, "objectClass": {"person"}})
						}
						op := ber.Encode(ber.ClassApplication, ber.TypeConstructed, 4, nil, "")
						op.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, entry.DN, "DN"))
						attrs := ber.NewSequence("attributes")
						for _, attr := range entry.Attributes {
							p := ber.NewSequence("attribute")
							p.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, attr.Name, "name"))
							values := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSet, nil, "")
							for _, v := range attr.Values {
								values.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, v, "value"))
							}
							p.AppendChild(values)
							attrs.AppendChild(p)
						}
						op.AppendChild(attrs)
						response(op)
						code := int64(0)
						if partial.Load() {
							code = ldap.LDAPResultSizeLimitExceeded
						}
						done(5, code)
					case 2:
						return
					}
				}
			}()
		}
	}()
	dc.URL, dc.BindDN, dc.BindPassword = "ldap://"+listener.Addr().String(), "cn=reader,dc=example,dc=com", "test-password"
	d := NewDirectory(store, config.LDAPConfig{Domains: map[string]config.LDAPDomainConfig{domain.Name: dc}})
	preview, err := d.LoadOrganization(ctx, domain.ID, false)
	if err != nil || len(preview.Groups) != 1 || preview.Memberships != 1 {
		t.Fatal(preview, err)
	}
	groups, _ := store.ListDirectoryGroups(ctx, domain.ID)
	if len(groups) != 0 {
		t.Fatal("preview wrote groups")
	}
	applied, err := d.LoadOrganization(ctx, domain.ID, true)
	if err != nil || applied.Revision <= current.Revision {
		t.Fatal(applied, err)
	}
	recipients, err := store.ResolveRecipients(ctx, "team@example.com")
	if err != nil || len(recipients) != 1 || recipients[0].ID != user.ID {
		t.Fatal(recipients, err)
	}
	partial.Store(true)
	if _, err = d.LoadOrganization(ctx, domain.ID, true); err == nil {
		t.Fatal("accepted incomplete LDAP search")
	}
	groups, _ = store.ListDirectoryGroups(ctx, domain.ID)
	current, _ = store.GetDirectorySource(ctx, domain.ID)
	if len(groups) != 1 || current.LastError == "" || current.LastSuccess.IsZero() || current.Revision != applied.Revision+1 {
		t.Fatal(groups, current)
	}
	actual, _ := store.GetUserByID(ctx, user.ID)
	if actual.Roles != "user" || actual.AuthSource != "local" {
		t.Fatal("import changed authentication or permissions", actual)
	}
	select {
	case err := <-errs:
		t.Fatal(err)
	default:
	}
}
