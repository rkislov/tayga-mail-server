package dav

import (
	"net/url"
	"path"
	"strings"
)

const (
	calPrefix  = "/dav/cal"
	cardPrefix = "/dav/card"
)

func calPrincipal(email string) string {
	return calPrefix + "/" + email + "/"
}

func calHome(email string) string {
	return calPrincipal(email) + "calendars/"
}

func calCollection(email, name string) string {
	return calHome(email) + name + "/"
}

func calObjectPath(email, calName, href string) string {
	return calCollection(email, calName) + href
}

func cardPrincipal(email string) string {
	return cardPrefix + "/" + email + "/"
}

func cardHome(email string) string {
	return cardPrincipal(email) + "addressbooks/"
}

func cardCollection(email, name string) string {
	return cardHome(email) + name + "/"
}

func cardObjectPath(email, abName, href string) string {
	return cardCollection(email, abName) + href
}

// parseCalPath extracts email, calendar name, and object href from a CalDAV URL.
// Depths (after /dav/cal): principal / home / calendar / object.
func parseCalPath(p string) (email, calName, href string, ok bool) {
	p = normalizeDAVPath(p, calPrefix)
	parts := splitPath(p)
	if len(parts) < 1 {
		return "", "", "", false
	}
	email, err := url.PathUnescape(parts[0])
	if err != nil {
		email = parts[0]
	}
	email = strings.ToLower(email)
	if len(parts) == 1 {
		return email, "", "", true
	}
	if parts[1] != "calendars" {
		return "", "", "", false
	}
	if len(parts) == 2 {
		return email, "", "", true
	}
	calName = parts[2]
	if len(parts) == 3 {
		return email, calName, "", true
	}
	href = parts[3]
	return email, calName, href, true
}

func parseCardPath(p string) (email, abName, href string, ok bool) {
	p = normalizeDAVPath(p, cardPrefix)
	parts := splitPath(p)
	if len(parts) < 1 {
		return "", "", "", false
	}
	email, err := url.PathUnescape(parts[0])
	if err != nil {
		email = parts[0]
	}
	email = strings.ToLower(email)
	if len(parts) == 1 {
		return email, "", "", true
	}
	if parts[1] != "addressbooks" {
		return "", "", "", false
	}
	if len(parts) == 2 {
		return email, "", "", true
	}
	abName = parts[2]
	if len(parts) == 3 {
		return email, abName, "", true
	}
	href = parts[3]
	return email, abName, href, true
}

func normalizeDAVPath(p, prefix string) string {
	p = path.Clean("/" + strings.TrimPrefix(p, "/"))
	p = strings.TrimPrefix(p, prefix)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
