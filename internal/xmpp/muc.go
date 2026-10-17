// SPDX-License-Identifier: Apache-2.0
package xmpp

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

const mucNS = "http://jabber.org/protocol/muc"
const discoInfo = "http://jabber.org/protocol/disco#info"
const discoItems = "http://jabber.org/protocol/disco#items"

type mucOccupant struct{ Nick, Affiliation, Show, Status string }
type mucResumeState struct {
	JID, Nick string
	Since     time.Time
}
type mucRoom struct{ JID, Name, Owner, Subject string }

func (s *Server) getRoom(ctx context.Context, jid string) (mucRoom, error) {
	var r mucRoom
	r.JID = jid
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT name,owner_email,subject FROM web_chat_rooms WHERE jid=?`), jid).Scan(&r.Name, &r.Owner, &r.Subject)
	return r, err
}
func (s *Server) roomAffiliation(ctx context.Context, room mucRoom, email string) string {
	if err := s.requireRoomMember(ctx, email, room.JID); err != nil {
		return "none"
	}
	if room.Owner == email {
		return "owner"
	}
	var affiliation string
	if s.db.QueryRowContext(ctx, s.rebind(`SELECT affiliation FROM web_chat_members WHERE room_jid=? AND email=?`), room.JID, email).Scan(&affiliation) != nil {
		return "none"
	}
	return affiliation
}
func mucRole(affiliation string) string {
	if affiliation == "owner" || affiliation == "admin" {
		return "moderator"
	}
	return "participant"
}
func (s *Session) mucError(kind, id, to, condition string) error {
	return s.writeString(fmt.Sprintf(`<%s from='%s' to='%s'%s type='error'><error type='cancel'><%s xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></%s>`, kind, xmlEscape(to), xmlEscape(s.JID.Full()), attrIf("id", id), condition, kind))
}
func (s *Session) mucResult(id, to, payload string) error {
	return s.writeString(fmt.Sprintf(`<iq from='%s' to='%s' id='%s' type='result'>%s</iq>`, xmlEscape(to), xmlEscape(s.JID.Full()), xmlEscape(id), payload))
}
func (s *Server) inviteRoomMember(from, to, room string) {
	stanza := fmt.Sprintf(`<message from='%s' to='%s'><x xmlns='%s#user'><invite from='%s'/></x></message>`, xmlEscape(room), xmlEscape(to), mucNS, xmlEscape(from))
	if !s.hub.Route(to, []byte(stanza)) {
		_ = s.storeOffline(context.Background(), to, stanza)
	}
}
func mucPresence(room string, session *Session, occupant mucOccupant, target string, unavailable bool, codes string) string {
	typ := ""
	role := mucRole(occupant.Affiliation)
	if unavailable {
		typ = " type='unavailable'"
		role = "none"
	}
	payload := ""
	if occupant.Show != "" {
		payload += "<show>" + xmlEscape(occupant.Show) + "</show>"
	}
	if occupant.Status != "" {
		payload += "<status>" + xmlEscape(occupant.Status) + "</status>"
	}
	return fmt.Sprintf(`<presence from='%s/%s' to='%s'%s>%s<x xmlns='%s#user'><item affiliation='%s' role='%s' jid='%s'/>%s</x></presence>`, xmlEscape(room), xmlEscape(occupant.Nick), xmlEscape(target), typ, payload, mucNS, xmlEscape(occupant.Affiliation), role, xmlEscape(session.JID.Full()), codes)
}

// Several resources may share one occupant nickname. A disconnect must not
// announce that nickname as offline while another resource remains joined.
func mucRemaining(occupants map[*Session]mucOccupant, nick string) (*Session, mucOccupant) {
	for session, occupant := range occupants {
		if occupant.Nick == nick {
			return session, occupant
		}
	}
	return nil, mucOccupant{}
}
func mucDeparture(room string, session *Session, occupant mucOccupant, occupants map[*Session]mucOccupant) {
	replacement, remaining := mucRemaining(occupants, occupant.Nick)
	for other := range occupants {
		if replacement != nil {
			other.SendRaw([]byte(mucPresence(room, replacement, remaining, other.JID.Full(), false, "")))
		} else {
			other.SendRaw([]byte(mucPresence(room, session, occupant, other.JID.Full(), true, "")))
		}
	}
}
func (s *Server) evictRoomMember(room, email, code string) {
	for session, occupant := range s.muc[room] {
		if session.JID.Bare() != email {
			continue
		}
		delete(s.muc[room], session)
		codes := `<status code='` + code + `'/>`
		session.SendRaw([]byte(mucPresence(room, session, occupant, session.JID.Full(), true, codes+`<status code='110'/>`)))
		for other := range s.muc[room] {
			other.SendRaw([]byte(mucPresence(room, session, occupant, other.JID.Full(), true, codes)))
		}
	}
}
func (s *Session) leaveMUC() {
	srv := s.srv
	srv.mucMu.Lock()
	defer srv.mucMu.Unlock()
	for room, occupants := range srv.muc {
		if occupant, ok := occupants[s]; ok {
			s.mu.Lock()
			if s.sm.resumable && srv.mucPending[room] != s {
				s.mucResume = append(s.mucResume, mucResumeState{room, occupant.Nick, time.Now().UTC()})
			}
			s.mu.Unlock()
			delete(occupants, s)
			mucDeparture(room, s, occupant, occupants)
		}
		if len(occupants) == 0 {
			delete(srv.muc, room)
		}
		if srv.mucPending[room] == s {
			_, _ = srv.db.ExecContext(context.Background(), srv.rebind(`DELETE FROM web_chat_rooms WHERE jid=?`), room)
			delete(srv.mucPending, room)
		}
	}
}
func (s *Session) handleMUCPresence(to, typ, inner string) error {
	srv := s.srv
	srv.mucMu.Lock()
	defer srv.mucMu.Unlock()
	ctx := context.Background()
	jid := ParseJID(to)
	fail := func(condition string) error { return s.mucError("presence", "", to, condition) }
	if jid.Local == "" || jid.Resource == "" || len(jid.Resource) > 256 {
		return fail("jid-malformed")
	}
	if typ == "error" {
		return nil
	}
	if typ != "" && typ != "unavailable" {
		return fail("bad-request")
	}
	room, err := srv.getRoom(ctx, jid.Bare())
	created := false
	if err == sql.ErrNoRows && typ == "" {
		var req struct {
			X struct{ XMLName xml.Name } `xml:"x"`
		}
		if xml.Unmarshal([]byte("<presence>"+inner+"</presence>"), &req) != nil || req.X.XMLName.Space != mucNS {
			return fail("item-not-found")
		}
		if _, err = srv.createChatRoom(ctx, s.JID.Bare(), jid.Local, nil, jid.Bare()); err != nil {
			return fail("not-allowed")
		}
		room, err = srv.getRoom(ctx, jid.Bare())
		created = true
		if srv.mucPending == nil {
			srv.mucPending = map[string]*Session{}
		}
		srv.mucPending[jid.Bare()] = s
	}
	if err != nil {
		return fail("item-not-found")
	}
	affiliation := srv.roomAffiliation(ctx, room, s.JID.Bare())
	if affiliation == "none" {
		return fail("registration-required")
	}
	if owner := srv.mucPending[room.JID]; owner != nil && owner != s {
		return fail("item-not-found")
	}
	if srv.muc == nil {
		srv.muc = map[string]map[*Session]mucOccupant{}
	}
	if srv.muc[room.JID] == nil {
		srv.muc[room.JID] = map[*Session]mucOccupant{}
	}
	occupants := srv.muc[room.JID]
	if typ == "unavailable" {
		old, ok := occupants[s]
		if !ok {
			return nil
		}
		delete(occupants, s)
		s.SendRaw([]byte(mucPresence(room.JID, s, old, s.JID.Full(), true, `<status code='110'/>`)))
		mucDeparture(room.JID, s, old, occupants)
		if srv.mucPending[room.JID] == s {
			_, _ = srv.db.ExecContext(ctx, srv.rebind(`DELETE FROM web_chat_rooms WHERE jid=?`), room.JID)
			delete(srv.mucPending, room.JID)
		}
		return nil
	}
	for other, occupant := range occupants {
		if other != s && occupant.Nick == jid.Resource && other.JID.Bare() != s.JID.Bare() {
			return fail("conflict")
		}
	}
	if len(occupants) >= 200 {
		if _, ok := occupants[s]; !ok {
			return fail("resource-constraint")
		}
	}
	var req struct {
		Show   string `xml:"show"`
		Status string `xml:"status"`
		X      struct {
			History struct {
				Max     *int   `xml:"maxstanzas,attr"`
				Chars   *int   `xml:"maxchars,attr"`
				Seconds *int   `xml:"seconds,attr"`
				Since   string `xml:"since,attr"`
			} `xml:"history"`
		} `xml:"http://jabber.org/protocol/muc x"`
	}
	if xml.Unmarshal([]byte("<presence>"+inner+"</presence>"), &req) != nil {
		return fail("bad-request")
	}
	limit := 50
	if req.X.History.Max != nil {
		limit = *req.X.History.Max
	}
	if limit < 0 || req.X.History.Chars != nil && *req.X.History.Chars < 0 || req.X.History.Seconds != nil && *req.X.History.Seconds < 0 {
		return fail("bad-request")
	}
	if limit > 200 {
		limit = 200
	}
	since := time.Time{}
	if req.X.History.Since != "" {
		since, err = time.Parse(time.RFC3339Nano, req.X.History.Since)
		if err != nil {
			return fail("bad-request")
		}
	}
	if req.X.History.Seconds != nil {
		v := time.Now().Add(-time.Duration(*req.X.History.Seconds) * time.Second)
		if v.After(since) {
			since = v
		}
	}
	if old, ok := occupants[s]; ok && old.Nick != jid.Resource {
		delete(occupants, s)
		remaining, _ := mucRemaining(occupants, old.Nick)
		occupants[s] = old
		for other := range occupants {
			if remaining != nil && other != s {
				continue
			}
			codes := `<status code='303'/>`
			if other == s {
				codes += `<status code='110'/>`
			}
			raw := mucPresence(room.JID, s, old, other.JID.Full(), true, codes)
			raw = strings.Replace(raw, "<item ", "<item nick='"+xmlEscape(jid.Resource)+"' ", 1)
			other.SendRaw([]byte(raw))
		}
	}
	_, already := occupants[s]
	occupant := mucOccupant{Nick: jid.Resource, Affiliation: affiliation, Show: req.Show, Status: req.Status}
	if !already {
		seen := map[string]bool{}
		for other, o := range occupants {
			if seen[o.Nick] {
				continue
			}
			seen[o.Nick] = true
			s.SendRaw([]byte(mucPresence(room.JID, other, o, s.JID.Full(), false, "")))
		}
	}
	occupants[s] = occupant
	for other := range occupants {
		codes := ""
		if other == s {
			codes = `<status code='110'/>`
			if created {
				codes += `<status code='201'/>`
			}
		}
		other.SendRaw([]byte(mucPresence(room.JID, s, occupant, other.JID.Full(), false, codes)))
	}
	if !already && limit > 0 {
		history, e := srv.roomHistory(ctx, s.JID.Bare(), room.JID, limit)
		if e != nil {
			return fail("internal-server-error")
		}
		chars := 0
		for _, message := range history {
			if message.At.Before(since) {
				continue
			}
			nick := message.Nick
			if nick == "" {
				nick = message.From
			}
			raw := roomMessageXML(room.JID, nick, s.JID.Full(), message.ID, message.Body, message.OriginID, fmt.Sprintf(`<delay xmlns='urn:xmpp:delay' from='%s' stamp='%s'/>`, xmlEscape(room.JID), message.At.UTC().Format(time.RFC3339Nano)))
			chars += len([]rune(raw))
			if req.X.History.Chars != nil && chars > *req.X.History.Chars {
				break
			}
			s.SendRaw([]byte(raw))
		}
	}
	if !already {
		s.SendRaw([]byte(fmt.Sprintf(`<message from='%s' to='%s' type='groupchat'><subject>%s</subject></message>`, xmlEscape(room.JID), xmlEscape(s.JID.Full()), xmlEscape(room.Subject))))
	}
	return nil
}
func roomMessageXML(room, nick, to, id, body, origin, extra string) string {
	if origin != "" {
		extra += `<origin-id xmlns='urn:xmpp:sid:0' id='` + xmlEscape(origin) + `'/>`
	}
	return fmt.Sprintf(`<message from='%s/%s' to='%s' type='groupchat' id='%s'><body>%s</body><stanza-id xmlns='urn:xmpp:sid:0' by='%s' id='%s'/>%s</message>`, xmlEscape(room), xmlEscape(nick), xmlEscape(to), xmlEscape(id), xmlEscape(body), xmlEscape(room), xmlEscape(id), extra)
}
func (s *Server) broadcastRoomMessage(ev ChatEvent, nick, origin string) {
	for session := range s.muc[ev.Room] {
		session.SendRaw([]byte(roomMessageXML(ev.Room, nick, session.JID.Full(), ev.ID, ev.Body, origin, "")))
	}
}
func (s *Session) handleMUCMessage(to, typ, id, inner string) error {
	srv := s.srv
	srv.mucMu.Lock()
	defer srv.mucMu.Unlock()
	ctx := context.Background()
	jid := ParseJID(to)
	fail := func(condition string) error { return s.mucError("message", id, to, condition) }
	if typ == "error" {
		return nil
	}
	if jid.Local == "" || jid.Resource != "" {
		return fail("not-allowed")
	}
	room, err := srv.getRoom(ctx, jid.Bare())
	if err != nil {
		return fail("item-not-found")
	}
	aff := srv.roomAffiliation(ctx, room, s.JID.Bare())
	if aff == "none" {
		return fail("forbidden")
	}
	var payload struct {
		Body     string                       `xml:"body"`
		Subject  *string                      `xml:"subject"`
		Children []struct{ XMLName xml.Name } `xml:",any"`
		Origin   struct {
			ID string `xml:"id,attr"`
		} `xml:"urn:xmpp:sid:0 origin-id"`
		X struct {
			Invite *struct {
				To string `xml:"to,attr"`
			} `xml:"invite"`
		} `xml:"http://jabber.org/protocol/muc#user x"`
	}
	if xml.Unmarshal([]byte("<message>"+inner+"</message>"), &payload) != nil {
		return fail("bad-request")
	}
	if payload.X.Invite != nil {
		if aff != "owner" && aff != "admin" {
			return fail("forbidden")
		}
		target := ParseJID(payload.X.Invite.To).Bare()
		if err = srv.setRoomAffiliation(ctx, s.JID.Bare(), room, target, "member"); err != nil {
			return fail("not-allowed")
		}
		srv.inviteRoomMember(s.JID.Bare(), target, room.JID)
		return nil
	}
	if typ != "groupchat" {
		return fail("bad-request")
	}
	occupant, ok := srv.muc[room.JID][s]
	if !ok {
		return fail("not-acceptable")
	}
	for _, child := range payload.Children {
		if child.XMLName.Local == "encrypted" {
			return fail("feature-not-implemented")
		}
	}
	if payload.Subject != nil {
		if aff != "owner" && aff != "admin" {
			return fail("forbidden")
		}
		if len(*payload.Subject) > 4096 {
			return fail("not-acceptable")
		}
		_, err = srv.db.ExecContext(ctx, srv.rebind(`UPDATE web_chat_rooms SET subject=? WHERE jid=?`), *payload.Subject, room.JID)
		if err != nil {
			return fail("internal-server-error")
		}
		for other := range srv.muc[room.JID] {
			other.SendRaw([]byte(fmt.Sprintf(`<message from='%s/%s' to='%s' type='groupchat'><subject>%s</subject></message>`, xmlEscape(room.JID), xmlEscape(occupant.Nick), xmlEscape(other.JID.Full()), xmlEscape(*payload.Subject))))
		}
		return nil
	}
	if payload.Body == "" {
		return nil
	}
	origin := payload.Origin.ID
	if origin == "" {
		origin = id
	}
	if len(origin) > 256 {
		return fail("bad-request")
	}
	if origin != "" {
		var oldID, body, nick string
		err = srv.db.QueryRowContext(ctx, srv.rebind(`SELECT id,body,nick FROM web_chat_messages WHERE room_jid=? AND sender=? AND origin_id=?`), room.JID, s.JID.Bare(), origin).Scan(&oldID, &body, &nick)
		if err == nil {
			return s.writeString(roomMessageXML(room.JID, nick, s.JID.Full(), oldID, body, origin, ""))
		}
		if err != sql.ErrNoRows {
			return fail("internal-server-error")
		}
	}
	if _, err = srv.sendRoomMessageAs(ctx, s.JID.Bare(), room.JID, payload.Body, occupant.Nick, origin); err != nil {
		return fail("internal-server-error")
	}
	return nil
}

func (s *Server) setRoomAffiliation(ctx context.Context, actor string, room mucRoom, target, aff string) error {
	actorAff := s.roomAffiliation(ctx, room, actor)
	if actorAff != "owner" && actorAff != "admin" {
		return fmt.Errorf("room administrator required")
	}
	if aff != "member" && aff != "admin" && aff != "none" && aff != "owner" {
		return fmt.Errorf("unsupported affiliation")
	}
	old := s.roomAffiliation(ctx, room, target)
	if target == room.Owner || actorAff != "owner" && (old == "admin" || aff == "admin" || aff == "owner") {
		return fmt.Errorf("owner action required")
	}
	targetUser, err := s.store.GetUserByEmail(ctx, target)
	ownerUser, e := s.store.GetUserByEmail(ctx, room.Owner)
	if err != nil || e != nil || !targetUser.Enabled || targetUser.TenantID != ownerUser.TenantID || targetUser.AuthSource == "resource" {
		return fmt.Errorf("participant unavailable")
	}
	if aff != "none" && old == "none" {
		members, e := s.roomMembers(ctx, room.JID)
		if e != nil {
			return e
		}
		if len(members) >= 100 {
			return fmt.Errorf("maximum 100 participants")
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if aff == "none" {
		_, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM web_chat_members WHERE room_jid=? AND email=?`), room.JID, target)
	} else {
		stored := aff
		if stored == "owner" {
			stored = "member"
		}
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO web_chat_members(room_jid,email,affiliation) VALUES(?,?,?) ON CONFLICT(room_jid,email) DO UPDATE SET affiliation=excluded.affiliation`), room.JID, target, stored)
		if err == nil && aff == "owner" {
			_, err = tx.ExecContext(ctx, s.rebind(`UPDATE web_chat_rooms SET owner_email=? WHERE jid=?`), target, room.JID)
			if err == nil {
				_, err = tx.ExecContext(ctx, s.rebind(`UPDATE web_chat_members SET affiliation='member' WHERE room_jid=? AND email=?`), room.JID, room.Owner)
			}
		}
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if aff == "none" {
		s.evictRoomMember(room.JID, target, "321")
	} else {
		for session, occupant := range s.muc[room.JID] {
			if session.JID.Bare() == target {
				occupant.Affiliation = aff
				s.muc[room.JID][session] = occupant
			} else if aff == "owner" && session.JID.Bare() == room.Owner {
				occupant.Affiliation = "member"
				s.muc[room.JID][session] = occupant
			} else {
				continue
			}
			for other := range s.muc[room.JID] {
				codes := ""
				if other == session {
					codes = `<status code='110'/>`
				}
				other.SendRaw([]byte(mucPresence(room.JID, session, occupant, other.JID.Full(), false, codes)))
			}
		}
	}
	s.hub.pushWeb(target, ChatEvent{Type: "roster"})
	s.hub.pushWeb(actor, ChatEvent{Type: "roster"})
	return nil
}
func (s *Server) destroyRoom(ctx context.Context, actor, jid string) error {
	room, err := s.getRoom(ctx, jid)
	if err != nil {
		return err
	}
	if room.Owner != actor {
		return fmt.Errorf("owner required")
	}
	members, err := s.roomMembers(ctx, jid)
	if err != nil {
		return err
	}
	if _, err = s.db.ExecContext(ctx, s.rebind(`DELETE FROM web_chat_rooms WHERE jid=?`), jid); err != nil {
		return err
	}
	for session, occupant := range s.muc[jid] {
		occupant.Affiliation = "none"
		session.SendRaw([]byte(mucPresence(jid, session, occupant, session.JID.Full(), true, `<destroy/><status code='110'/>`)))
	}
	delete(s.muc, jid)
	delete(s.mucPending, jid)
	for _, member := range members {
		s.hub.pushWeb(member, ChatEvent{Type: "roster"})
	}
	return nil
}

func (s *Session) handleMUCIQ(id, typ, to, inner string) (bool, error) {
	jid := ParseJID(to)
	if !s.srv.isWebRoom(to) {
		return false, nil
	}
	if typ == "error" || typ == "result" {
		return true, nil
	}
	srv := s.srv
	srv.mucMu.Lock()
	defer srv.mucMu.Unlock()
	ctx := context.Background()
	fail := func(c string) (bool, error) { return true, s.mucError("iq", id, to, c) }
	if jid.Resource != "" {
		return fail("feature-not-implemented")
	}
	var q struct {
		XMLName xml.Name
		Node    string `xml:"node,attr"`
		Item    []struct {
			JID         string `xml:"jid,attr"`
			Affiliation string `xml:"affiliation,attr"`
			Role        string `xml:"role,attr"`
			Nick        string `xml:"nick,attr"`
		} `xml:"item"`
		Destroy *struct{} `xml:"destroy"`
		X       struct {
			Type   string `xml:"type,attr"`
			Fields []struct {
				Var    string   `xml:"var,attr"`
				Values []string `xml:"value"`
			} `xml:"field"`
		} `xml:"jabber:x:data x"`
	}
	if xml.Unmarshal([]byte(inner), &q) != nil || q.Node != "" || q.XMLName.Local != "query" {
		return fail("bad-request")
	}
	if jid.Local == "" {
		if typ != "get" {
			return fail("not-allowed")
		}
		switch q.XMLName.Space {
		case discoInfo:
			return true, s.mucResult(id, to, `<query xmlns='`+discoInfo+`'><identity category='conference' type='text' name='Tayga groups'/><feature var='`+mucNS+`'/></query>`)
		case discoItems:
			rooms, err := srv.ListChatRooms(ctx, s.JID.Bare())
			if err != nil {
				return fail("internal-server-error")
			}
			var b strings.Builder
			for _, r := range rooms {
				b.WriteString(`<item jid='` + xmlEscape(r.JID) + `' name='` + xmlEscape(r.Name) + `'/>`)
			}
			return true, s.mucResult(id, to, `<query xmlns='`+discoItems+`'>`+b.String()+`</query>`)
		default:
			return fail("feature-not-implemented")
		}
	}
	room, err := srv.getRoom(ctx, jid.Bare())
	if err != nil {
		return fail("item-not-found")
	}
	aff := srv.roomAffiliation(ctx, room, s.JID.Bare())
	if aff == "none" {
		return fail("forbidden")
	}
	switch q.XMLName.Space {
	case discoInfo:
		if typ != "get" {
			return fail("bad-request")
		}
		return true, s.mucResult(id, to, `<query xmlns='`+discoInfo+`'><identity category='conference' type='text' name='`+xmlEscape(room.Name)+`'/><feature var='`+mucNS+`'/><feature var='muc_persistent'/><feature var='muc_membersonly'/><feature var='muc_nonanonymous'/><feature var='muc_unmoderated'/><feature var='muc_hidden'/><feature var='urn:xmpp:sid:0'/></query>`)
	case discoItems:
		if typ != "get" {
			return fail("bad-request")
		}
		var b strings.Builder
		seen := map[string]bool{}
		for _, occupant := range srv.muc[room.JID] {
			if seen[occupant.Nick] {
				continue
			}
			seen[occupant.Nick] = true
			b.WriteString(`<item jid='` + xmlEscape(room.JID+"/"+occupant.Nick) + `'/>`)
		}
		return true, s.mucResult(id, to, `<query xmlns='`+discoItems+`'>`+b.String()+`</query>`)
	case mucNS + "#admin":
		if typ == "get" {
			if len(q.Item) != 1 {
				return fail("bad-request")
			}
			filter := q.Item[0].Affiliation
			if filter != "member" && filter != "admin" && filter != "owner" {
				return fail("feature-not-implemented")
			}
			members, err := srv.roomMembers(ctx, room.JID)
			if err != nil {
				return fail("internal-server-error")
			}
			var b strings.Builder
			for _, member := range members {
				if srv.roomAffiliation(ctx, room, member) == filter {
					b.WriteString(`<item jid='` + xmlEscape(member) + `' affiliation='` + filter + `'/>`)
				}
			}
			return true, s.mucResult(id, to, `<query xmlns='`+mucNS+`#admin'>`+b.String()+`</query>`)
		}
		if typ != "set" || len(q.Item) != 1 {
			return fail("feature-not-implemented")
		}
		item := q.Item[0]
		if item.Role == "none" && item.Nick != "" {
			if aff != "owner" && aff != "admin" {
				return fail("forbidden")
			}
			for session, occupant := range srv.muc[room.JID] {
				if occupant.Nick == item.Nick {
					if occupant.Affiliation == "owner" || aff == "admin" && occupant.Affiliation == "admin" {
						return fail("not-allowed")
					}
					srv.evictRoomMember(room.JID, session.JID.Bare(), "307")
					return true, s.mucResult(id, to, "")
				}
			}
			return fail("item-not-found")
		}
		if item.Role != "" || item.JID == "" {
			return fail("bad-request")
		}
		target := ParseJID(item.JID).Bare()
		if err = srv.setRoomAffiliation(ctx, s.JID.Bare(), room, target, item.Affiliation); err != nil {
			return fail("not-allowed")
		}
		if item.Affiliation == "member" {
			srv.inviteRoomMember(s.JID.Bare(), target, room.JID)
		}
		return true, s.mucResult(id, to, "")
	case mucNS + "#owner":
		if aff != "owner" {
			return fail("forbidden")
		}
		if typ == "get" {
			return true, s.mucResult(id, to, `<query xmlns='`+mucNS+`#owner'><x xmlns='jabber:x:data' type='form'><field var='FORM_TYPE' type='hidden'><value>`+mucNS+`#roomconfig</value></field><field var='muc#roomconfig_roomname' type='text-single' label='Name'><value>`+xmlEscape(room.Name)+`</value></field><field var='muc#roomconfig_membersonly' type='hidden'><value>1</value></field><field var='muc#roomconfig_persistentroom' type='hidden'><value>1</value></field><field var='muc#roomconfig_whois' type='hidden'><value>anyone</value></field></x></query>`)
		}
		if typ != "set" {
			return fail("bad-request")
		}
		if q.Destroy != nil {
			if err = srv.destroyRoom(ctx, s.JID.Bare(), room.JID); err != nil {
				return fail("internal-server-error")
			}
			return true, s.mucResult(id, to, "")
		}
		if q.X.Type == "cancel" {
			if srv.mucPending[room.JID] == s {
				if err = srv.destroyRoom(ctx, s.JID.Bare(), room.JID); err != nil {
					return fail("internal-server-error")
				}
			}
			return true, s.mucResult(id, to, "")
		}
		if q.X.Type != "submit" {
			return fail("bad-request")
		}
		name := room.Name
		for _, f := range q.X.Fields {
			if len(f.Values) != 1 {
				return fail("not-acceptable")
			}
			v := f.Values[0]
			switch f.Var {
			case "FORM_TYPE":
				if v != mucNS+"#roomconfig" {
					return fail("bad-request")
				}
			case "muc#roomconfig_roomname":
				name = strings.TrimSpace(v)
				if name == "" || len([]rune(name)) > 100 {
					return fail("not-acceptable")
				}
			case "muc#roomconfig_membersonly", "muc#roomconfig_persistentroom":
				if v != "1" && v != "true" {
					return fail("not-acceptable")
				}
			case "muc#roomconfig_whois":
				if v != "anyone" {
					return fail("not-acceptable")
				}
			default:
				return fail("feature-not-implemented")
			}
		}
		if _, err = srv.db.ExecContext(ctx, srv.rebind(`UPDATE web_chat_rooms SET name=? WHERE jid=?`), name, room.JID); err != nil {
			return fail("internal-server-error")
		}
		delete(srv.mucPending, room.JID)
		return true, s.mucResult(id, to, "")
	default:
		return fail("feature-not-implemented")
	}
}

// Discovery is shared with the file service even if HTTP upload is disabled.
func (s *Session) serviceItems(id, typ, to, inner string) (bool, error) {
	name, ok := iqPayloadName(inner)
	if !ok || typ != "get" || name.Space != discoItems || name.Local != "query" || (to != "" && to != s.JID.Domain && to != s.srv.domain) {
		return false, nil
	}
	from := to
	if from == "" {
		from = s.JID.Domain
	}
	payload := `<query xmlns='` + discoItems + `'><item jid='web-groups.` + xmlEscape(s.srv.domain) + `' name='Tayga groups'/>`
	s.srv.uploadMu.RLock()
	upload := s.srv.upload != nil
	s.srv.uploadMu.RUnlock()
	if upload {
		payload += `<item jid='upload.` + xmlEscape(s.srv.domain) + `' name='Tayga file upload'/>`
	}
	return true, s.mucResult(id, from, payload+`</query>`)
}

// Web membership management follows the same owner/admin checks as XMPP.
func (s *Server) UpdateChatRoom(ctx context.Context, actor, jid, target, affiliation string, destroy bool) error {
	s.mucMu.Lock()
	defer s.mucMu.Unlock()
	if destroy {
		return s.destroyRoom(ctx, actor, jid)
	}
	room, err := s.getRoom(ctx, jid)
	if err != nil {
		return err
	}
	target = ParseJID(target).Bare()
	if err = s.setRoomAffiliation(ctx, actor, room, target, affiliation); err != nil {
		return err
	}
	if affiliation == "member" {
		s.inviteRoomMember(actor, target, jid)
	}
	return nil
}
