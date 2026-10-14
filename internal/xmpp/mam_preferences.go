package xmpp

import (
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"sort"
)

// MAM preferences apply to future archive writes, never to already stored history.
type mamPreferences struct {
	XMLName xml.Name `xml:"urn:xmpp:mam:2 prefs" json:"-"`
	Default string   `xml:"default,attr" json:"default"`
	Always  []string `xml:"always>jid" json:"always"`
	Never   []string `xml:"never>jid" json:"never"`
}

func (s *Server) mamPreferences(ctx context.Context, owner string) (mamPreferences, error) {
	p := mamPreferences{Default: "always"}
	if s.db == nil {
		return p, nil
	}
	var raw string
	err := s.db.QueryRowContext(ctx, s.rebind("SELECT preferences FROM xmpp_mam_preferences WHERE owner_bare_jid=?"), owner).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	err = json.Unmarshal([]byte(raw), &p)
	return p, err
}
func normalizeMAMPreferences(p *mamPreferences) error {
	if p.Default != "always" && p.Default != "never" && p.Default != "roster" {
		return errors.New("invalid archive default")
	}
	if len(p.Always)+len(p.Never) > 256 {
		return errors.New("too many archive rules")
	}
	seen := map[string]bool{}
	for _, list := range []*[]string{&p.Always, &p.Never} {
		for i, value := range *list {
			jid, err := NormalizeJID(value)
			if err != nil || jid.Domain == "" {
				return errors.New("invalid archive JID")
			}
			value = jid.Full()
			if seen[value] {
				return errors.New("duplicate archive rule")
			}
			seen[value] = true
			(*list)[i] = value
		}
		sort.Strings(*list)
	}
	return nil
}
func (s *Session) handleMAMPreferences(ctx context.Context, id, typ, inner string) error {
	if s.srv == nil || s.srv.db == nil || s.user == nil {
		return s.writeString(iqForbidden(id))
	}
	owner := s.JID.Bare()
	var p mamPreferences
	var err error
	switch typ {
	case "get":
		p, err = s.srv.mamPreferences(ctx, owner)
	case "set":
		if xml.Unmarshal([]byte(inner), &p) != nil || normalizeMAMPreferences(&p) != nil {
			return s.writeString(iqBadRequest(id))
		}
		raw, _ := json.Marshal(p)
		_, err = s.srv.db.ExecContext(ctx, s.srv.rebind("INSERT INTO xmpp_mam_preferences(owner_bare_jid,preferences) VALUES(?,?) ON CONFLICT(owner_bare_jid) DO UPDATE SET preferences=excluded.preferences"), owner, string(raw))
	default:
		return s.writeString(iqBadRequest(id))
	}
	if err != nil {
		return s.writeString(iqInternalError(id))
	}
	payload, err := xml.Marshal(p)
	if err != nil {
		return s.writeString(iqInternalError(id))
	}
	return s.writeString(fmt.Sprintf(`<iq type='result' id='%s' from='%s' to='%s'>%s</iq>`, xmlEscape(id), xmlEscape(owner), xmlEscape(s.JID.Full()), payload))
}
func (s *Server) shouldArchive(ctx context.Context, owner, peer, stanza string) (bool, error) {
	prefs, err := s.mamPreferences(ctx, owner)
	if err != nil {
		return false, err
	}
	// Match full resource exceptions against the original stanza, not the bare history peer.
	var message struct {
		From string `xml:"from,attr"`
		To   string `xml:"to,attr"`
	}
	if xml.Unmarshal([]byte(stanza), &message) == nil {
		if ParseJID(message.From).Bare() == owner {
			if message.To != "" {
				peer = message.To
			}
		} else if message.From != "" {
			peer = message.From
		}
	}
	jid, err := NormalizeJID(peer)
	if err != nil {
		return false, err
	}
	matches := func(list []string) bool {
		for _, value := range list {
			rule := ParseJID(value)
			if rule.IsFull() {
				if rule.Full() == jid.Full() {
					return true
				}
			} else if rule.Bare() == jid.Bare() {
				return true
			}
		}
		return false
	}
	if matches(prefs.Never) {
		return false, nil
	}
	if matches(prefs.Always) {
		return true, nil
	}
	switch prefs.Default {
	case "always":
		return true, nil
	case "never":
		return false, nil
	case "roster":
		var count int
		err := s.db.QueryRowContext(ctx, s.rebind("SELECT COUNT(*) FROM xmpp_roster r JOIN users u ON r.user_id=u.id WHERE u.email=? AND r.jid=?"), owner, jid.Bare()).Scan(&count)
		return count > 0, err
	}
	return false, errors.New("invalid stored archive preferences")
}
