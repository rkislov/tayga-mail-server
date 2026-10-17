package xmpp

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

const pushNamespace = "urn:xmpp:push:0"

// A configured authenticated component is an explicit delivery route. No public
// federation or vendor APNs access is implied by implementing this publisher.
func (s *Server) pushRoute(service string, connected bool) bool {
	if s.cfg == nil {
		return false
	}
	domain := ParseJID(service).Domain
	for _, c := range s.cfg.XMPP.Components {
		if c.Push && c.Secret != "" && s.componentDomain(c) == domain {
			return !connected || s.hub.Component(domain) != nil
		}
	}
	return s.pushS2SAllowed(service)
}
func (s *Session) pushDiscoveryFeature(from string) string {
	if s.user == nil || s.srv.cfg == nil || from != s.JID.Bare() {
		return ""
	}
	if len(s.srv.cfg.XMPP.PushServices) > 0 {
		if _, err := s.srv.pushClientCertificate(s.JID.Domain); err == nil {
			return `<feature var='urn:xmpp:push:0'/>`
		}
	}
	for _, c := range s.srv.cfg.XMPP.Components {
		if s.srv.pushRoute(s.srv.componentDomain(c), true) {
			return `<feature var='urn:xmpp:push:0'/>`
		}
	}
	return ""
}
func pushIQError(id, condition string) string {
	return fmt.Sprintf(`<iq type='error' id='%s'><error type='cancel'><%s xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></iq>`, xmlEscape(id), condition)
}
func (s *Session) handlePushIQ(id, typ, to, inner string) error {
	if s.user == nil || s.srv.db == nil {
		return s.writeString(iqAuthError(id))
	}
	if to != "" && to != s.JID.Bare() {
		return s.writeString(iqForbidden(id))
	}
	var command struct {
		XMLName  xml.Name
		JID      string  `xml:"jid,attr"`
		Node     *string `xml:"node,attr"`
		Children []struct {
			XMLName xml.Name
			Type    string `xml:"type,attr"`
			Fields  []struct {
				Var    string   `xml:"var,attr"`
				Values []string `xml:"value"`
			} `xml:"field"`
		} `xml:",any"`
	}
	if typ != "set" || len(inner) > 8192 || xml.Unmarshal([]byte(inner), &command) != nil || command.XMLName.Space != pushNamespace || (command.XMLName.Local != "enable" && command.XMLName.Local != "disable") {
		return s.writeString(iqBadRequest(id))
	}
	jid, err := NormalizeJID(command.JID)
	if err != nil || jid.Resource != "" || command.JID == "" {
		return s.writeString(iqBadRequest(id))
	}
	node := ""
	if command.Node != nil {
		node = *command.Node
	}
	if len(node) > 1024 {
		return s.writeString(iqBadRequest(id))
	}
	options := ""
	if len(command.Children) > 0 {
		if command.XMLName.Local != "enable" || len(command.Children) != 1 {
			return s.writeString(iqBadRequest(id))
		}
		form := command.Children[0]
		if form.XMLName != (xml.Name{Space: "jabber:x:data", Local: "x"}) || form.Type != "submit" || len(form.Fields) > 32 {
			return s.writeString(iqBadRequest(id))
		}
		formType := 0
		seen := map[string]bool{}
		var b strings.Builder
		b.WriteString(`<x xmlns='jabber:x:data' type='submit'>`)
		for _, field := range form.Fields {
			if field.Var == "" || seen[field.Var] || len(field.Values) == 0 || len(field.Values) > 16 {
				return s.writeString(iqBadRequest(id))
			}
			seen[field.Var] = true
			if field.Var == "FORM_TYPE" {
				if len(field.Values) != 1 || field.Values[0] != "http://jabber.org/protocol/pubsub#publish-options" {
					return s.writeString(iqBadRequest(id))
				}
				formType++
			}
			b.WriteString(`<field var='` + xmlEscape(field.Var) + `'>`)
			for _, v := range field.Values {
				b.WriteString(`<value>` + xmlEscape(v) + `</value>`)
			}
			b.WriteString(`</field>`)
		}
		if formType != 1 {
			return s.writeString(iqBadRequest(id))
		}
		b.WriteString(`</x>`)
		options = b.String()
	}
	srv := s.srv
	ctx := context.Background()
	srv.pushMu.Lock()
	defer srv.pushMu.Unlock()
	if command.XMLName.Local == "disable" {
		query := `DELETE FROM xmpp_push_registrations WHERE user_id=? AND service=?`
		args := []any{s.user.ID, jid.Bare()}
		if command.Node != nil {
			query += ` AND node=?`
			args = append(args, node)
		}
		if _, err = srv.db.ExecContext(ctx, srv.rebind(query), args...); err != nil {
			return s.writeString(iqInternalError(id))
		}
	} else {
		if !srv.pushRoute(jid.Bare(), true) {
			if srv.log != nil {
				srv.log.Warn("xmpp push route unavailable", "service", jid.Bare(), "account_domain", s.JID.Domain)
			}
			return s.writeString(pushIQError(id, "service-unavailable"))
		}
		if srv.hub.Component(jid.Domain) == nil {
			if _, e := srv.pushClientCertificate(s.JID.Domain); e != nil {
				return s.writeString(pushIQError(id, "service-unavailable"))
			}
		}
		var count, existing int
		if err = srv.db.QueryRowContext(ctx, srv.rebind(`SELECT COUNT(*) FROM xmpp_push_registrations WHERE user_id=?`), s.user.ID).Scan(&count); err != nil {
			return s.writeString(iqInternalError(id))
		}
		if err = srv.db.QueryRowContext(ctx, srv.rebind(`SELECT COUNT(*) FROM xmpp_push_registrations WHERE user_id=? AND service=? AND node=?`), s.user.ID, jid.Bare(), node).Scan(&existing); err != nil {
			return s.writeString(iqInternalError(id))
		}
		if count >= 20 && existing == 0 {
			return s.writeString(pushIQError(id, "resource-constraint"))
		}
		// Replace the registration identity atomically. Old in-flight acknowledgements
		// cannot delete a renewed registration or its new queue job.
		tx, e := srv.db.BeginTx(ctx, nil)
		if e != nil {
			return s.writeString(iqInternalError(id))
		}
		defer tx.Rollback()
		_, e = tx.ExecContext(ctx, srv.rebind(`DELETE FROM xmpp_push_registrations WHERE user_id=? AND service=? AND node=?`), s.user.ID, jid.Bare(), node)
		if e == nil {
			_, e = tx.ExecContext(ctx, srv.rebind(`INSERT INTO xmpp_push_registrations(id,user_id,owner,service,node,options_xml,resource) VALUES(?,?,?,?,?,?,?)`), storage.NewID(), s.user.ID, s.JID.Bare(), jid.Bare(), node, options, s.JID.Resource)
		}
		if e == nil {
			e = tx.Commit()
		}
		if e != nil {
			return s.writeString(iqInternalError(id))
		}
	}
	if srv.log != nil {
		srv.log.Info("xmpp push registration updated", "operation", command.XMLName.Local, "service", jid.Bare(), "account", s.JID.Bare())
	}
	return s.writeString(fmt.Sprintf(`<iq type='result' id='%s'/>`, xmlEscape(id)))
}
func (s *Server) enqueueNativePush(owner string, ev ChatEvent) {
	if s.db == nil || ev.Type != "message" || (ev.Body == "" && ev.Attachment == nil && !ev.Encrypted) || ParseJID(ev.From).Bare() == owner {
		return
	}
	ctx := context.Background()
	s.pushMu.Lock()
	defer s.pushMu.Unlock()
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id,resource FROM xmpp_push_registrations WHERE owner=?`), owner)
	if err != nil {
		return
	}
	type registration struct{ id, resource string }
	var registrations []registration
	for rows.Next() {
		var r registration
		if rows.Scan(&r.id, &r.resource) == nil {
			registrations = append(registrations, r)
		}
	}
	rows.Close()
	for _, r := range registrations {
		active := false
		for _, session := range s.hub.Sessions(owner) {
			session.mu.Lock()
			active = session.JID.Resource == r.resource && !session.closed && !session.csi.inactive
			session.mu.Unlock()
			if active {
				break
			}
		}
		if active {
			continue
		}
		// Coalesce: at most one job per registration, bounded globally. A generation
		// protects new arrivals while a previous notification awaits acknowledgement.
		_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_push_jobs(registration_id) SELECT ? WHERE EXISTS(SELECT 1 FROM xmpp_push_jobs WHERE registration_id=?) OR (SELECT COUNT(*) FROM xmpp_push_jobs)<10000 ON CONFLICT(registration_id) DO UPDATE SET generation=xmpp_push_jobs.generation+1`), r.id, r.id)
		if err != nil && s.log != nil {
			s.log.Warn("xmpp push enqueue failed", "error", err)
		}
	}
}
func (s *Server) runPushWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.deliverPushBatch(ctx, time.Now())
		}
	}
}
func (s *Server) deliverPushBatch(ctx context.Context, now time.Time) {
	if s.db == nil {
		return
	}
	// A single worker per database. The durable request ID also rejects delayed
	// acknowledgements after timeout/restart or a replacement registration.
	s.pushMu.Lock()
	type job struct {
		id, owner, service, node, options, user string
		generation                              int64
		attempts                                int
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT r.id,r.owner,r.service,r.node,r.options_xml,r.user_id,j.generation,j.attempts FROM xmpp_push_jobs j JOIN xmpp_push_registrations r ON r.id=j.registration_id WHERE j.next_attempt<=? ORDER BY j.next_attempt LIMIT 100`), now.Unix())
	if err != nil {
		s.pushMu.Unlock()
		return
	}
	var jobs []job
	for rows.Next() {
		var j job
		if rows.Scan(&j.id, &j.owner, &j.service, &j.node, &j.options, &j.user, &j.generation, &j.attempts) == nil {
			jobs = append(jobs, j)
		}
	}
	rows.Close()
	s.pushMu.Unlock()
	s2sCount := 0
	for _, j := range jobs {
		s.pushMu.Lock()
		// The registration may have been disabled or replaced since the snapshot.
		var currentGeneration int64
		var due int64
		e := s.db.QueryRowContext(ctx, s.rebind(`SELECT generation,next_attempt FROM xmpp_push_jobs WHERE registration_id=?`), j.id).Scan(&currentGeneration, &due)
		if e != nil || due > now.Unix() {
			s.pushMu.Unlock()
			continue
		}
		user, e := s.store.GetUserByID(ctx, j.user)
		if e != nil || !user.Enabled || j.attempts >= 5 {
			_, _ = s.db.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_push_registrations WHERE id=?`), j.id)
			s.pushMu.Unlock()
			continue
		}
		if !s.pushRoute(j.service, true) {
			_, _ = s.db.ExecContext(ctx, s.rebind(`UPDATE xmpp_push_jobs SET next_attempt=? WHERE registration_id=?`), now.Add(time.Minute).Unix(), j.id)
			s.pushMu.Unlock()
			continue
		}
		component := s.hub.Component(ParseJID(j.service).Domain)
		if component == nil && s2sCount >= 4 {
			s.pushMu.Unlock()
			continue
		}
		requestID := "push-" + storage.NewID()
		delay := time.Duration(30*(1<<j.attempts)) * time.Second
		_, e = s.db.ExecContext(ctx, s.rebind(`UPDATE xmpp_push_jobs SET request_id=?,published_generation=?,attempts=attempts+1,next_attempt=? WHERE registration_id=?`), requestID, currentGeneration, now.Add(delay).Unix(), j.id)
		if e != nil {
			s.pushMu.Unlock()
			continue
		}
		options := ""
		if j.options != "" {
			options = `<publish-options>` + j.options + `</publish-options>`
		}
		raw := []byte(fmt.Sprintf(`<iq from='%s' to='%s' id='%s' type='set'><pubsub xmlns='http://jabber.org/protocol/pubsub'><publish node='%s'><item><notification xmlns='urn:xmpp:push:0'/></item></publish>%s</pubsub></iq>`, xmlEscape(j.owner), xmlEscape(j.service), requestID, xmlEscape(j.node), options))
		// Serialize disable with publishing. Component writes are bounded by a deadline;
		// no plaintext, sender, attachment URL or OMEMO key is in the notification.
		if component != nil {
			s.hub.Route(j.service, raw)
			s.pushMu.Unlock()
		} else {
			s.pushMu.Unlock()
			s2sCount++
			if err := s.publishPushS2S(ctx, j.owner, j.service, requestID, raw); err != nil && s.log != nil {
				s.log.Warn("xmpp push delivery failed", "request_id", requestID, "service", j.service, "error", err)
			}
		}
		if s.log != nil {
			s.log.Info("xmpp push attempt", "request_id", requestID, "service", j.service, "attempt", j.attempts+1)
		}
	}
}
func (s *Server) acceptPushReply(from, to, id, typ, inner string) bool {
	if !strings.HasPrefix(id, "push-") || (typ != "result" && typ != "error") || s.db == nil {
		return false
	}
	s.pushMu.Lock()
	defer s.pushMu.Unlock()
	ctx := context.Background()
	var registration, service, owner string
	var generation, published int64
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT r.id,r.service,r.owner,j.generation,j.published_generation FROM xmpp_push_jobs j JOIN xmpp_push_registrations r ON r.id=j.registration_id WHERE j.request_id=?`), id).Scan(&registration, &service, &owner, &generation, &published)
	if err != nil || from != service || to != owner {
		return true
	}
	if typ == "error" {
		var fault struct {
			Type string `xml:"type,attr"`
		}
		var wrapper struct {
			Error struct {
				Type string `xml:"type,attr"`
			} `xml:"error"`
		}
		_ = xml.Unmarshal([]byte("<root>"+inner+"</root>"), &wrapper)
		fault.Type = wrapper.Error.Type
		if fault.Type != "wait" {
			_, _ = s.db.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_push_registrations WHERE id=?`), registration)
		}
		if s.log != nil {
			s.log.Warn("xmpp push rejected", "request_id", id, "service", service, "temporary", fault.Type == "wait")
		}
		return true
	}
	if generation == published {
		_, _ = s.db.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_push_jobs WHERE registration_id=?`), registration)
	} else {
		_, _ = s.db.ExecContext(ctx, s.rebind(`UPDATE xmpp_push_jobs SET request_id='',attempts=0,next_attempt=0 WHERE registration_id=?`), registration)
	}
	return true
}
