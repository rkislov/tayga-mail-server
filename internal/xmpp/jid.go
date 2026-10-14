package xmpp

import (
	"fmt"
	"golang.org/x/net/idna"
	"golang.org/x/text/secure/precis"
	"strings"
)

// JID is a bare or full Jabber ID.
type JID struct {
	Local    string
	Domain   string
	Resource string
}

func ParseJID(s string) JID {
	s = strings.TrimSpace(s)
	var res string
	if i := strings.IndexByte(s, '/'); i >= 0 {
		res = s[i+1:]
		s = s[:i]
	}
	local, domain, ok := strings.Cut(s, "@")
	if !ok {
		return JID{Domain: strings.ToLower(s), Resource: res}
	}
	return JID{
		Local:    strings.ToLower(local),
		Domain:   strings.ToLower(domain),
		Resource: res,
	}
}

func (j JID) Bare() string {
	if j.Local == "" {
		return j.Domain
	}
	return j.Local + "@" + j.Domain
}

func (j JID) Full() string {
	b := j.Bare()
	if j.Resource == "" {
		return b
	}
	return b + "/" + j.Resource
}

func (j JID) IsFull() bool { return j.Resource != "" }

func EmailToBare(email string) string {
	return ParseJID(email).Bare()
}

// NormalizeJID validates protocol input without silently changing stored IDs.
// PRECIS profiles distinguish case-mapped usernames from case-sensitive resources.
func NormalizeJID(raw string) (JID, error) {
	var jid JID
	bare, resource, hasResource := strings.Cut(raw, "/")
	local, domain, hasLocal := strings.Cut(bare, "@")
	if !hasLocal {
		domain = bare
	}
	if domain == "" {
		return jid, fmt.Errorf("empty JID domain")
	}
	normalizedDomain, err := idna.Lookup.ToASCII(domain)
	if err != nil || normalizedDomain == "" || len(normalizedDomain) > 1023 || strings.ContainsAny(normalizedDomain, " /@\\\x00") {
		return jid, fmt.Errorf("invalid JID domain")
	}
	jid.Domain = strings.ToLower(normalizedDomain)
	if hasLocal {
		if local == "" || strings.ContainsAny(local, `"&'/:<>@`) {
			return JID{}, fmt.Errorf("invalid JID localpart")
		}
		jid.Local, err = precis.UsernameCaseMapped.String(local)
		if err != nil || len(jid.Local) == 0 || len(jid.Local) > 1023 || strings.ContainsAny(jid.Local, `"&'/:<>@`) {
			return JID{}, fmt.Errorf("invalid JID localpart")
		}
	}
	if hasResource {
		jid.Resource, err = precis.OpaqueString.String(resource)
		if err != nil || len(jid.Resource) == 0 || len(jid.Resource) > 1023 {
			return JID{}, fmt.Errorf("invalid JID resource")
		}
	}
	return jid, nil
}
