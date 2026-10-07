package xmpp

import (
	"strings"
)

// JID is a bare or full Jabber ID.
type JID struct {
	Local  string
	Domain string
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
