package xmpp

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/tayga/tms/internal/storage"
)

type Session struct {
	sessionID string
	srv       *Server
	log       *slog.Logger
	conn      net.Conn
	br        *bufio.Reader
	bw        *bufio.Writer
	dec       *xml.Decoder
	tlsCfg    *tls.Config

	mu       sync.Mutex
	closed   bool
	authed   bool
	bound    bool
	tlsDone  bool
	carbons  bool
	user     *storage.User
	JID      JID
	streamID string
}

func newSession(srv *Server, c net.Conn, alreadyTLS bool) *Session {
	br := bufio.NewReader(c)
	bw := bufio.NewWriter(c)
	return &Session{
		srv:      srv,
		log:      srv.log,
		conn:     c,
		br:       br,
		bw:       bw,
		dec:      xml.NewDecoder(br),
		tlsCfg:   srv.tlsCfg,
		tlsDone:  alreadyTLS,
		streamID: randomID(),
	}
}

func (s *Session) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.srv.authn.ForgetSession(s.sessionID)
	s.mu.Unlock()
	if s.bound {
		s.srv.hub.Unregister(s)
	}
	_ = s.conn.Close()
}

func (s *Session) SendRaw(b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	_, _ = s.bw.Write(b)
	_ = s.bw.Flush()
}

func (s *Session) writeString(str string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return net.ErrClosed
	}
	if _, err := s.bw.WriteString(str); err != nil {
		return err
	}
	return s.bw.Flush()
}

func (s *Session) serve() {
	defer s.Close()
	_ = s.conn.SetDeadline(time.Now().Add(5 * time.Minute))
	if err := s.handleStream(); err != nil && err != io.EOF {
		s.log.Debug("xmpp session ended", "err", err, "jid", s.JID.Full())
	}
}

func (s *Session) handleStream() error {
	for {
		tok, err := s.dec.Token()
		if err != nil {
			return err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local != "stream" {
			return fmt.Errorf("expected stream, got %s", se.Name.Local)
		}
		domain := attr(se, "to")
		if domain == "" {
			domain = s.srv.domain
		}
		if err := s.openStream(domain); err != nil {
			return err
		}
		if err := s.writeFeatures(); err != nil {
			return err
		}
		if err := s.negotiate(domain); err != nil {
			return err
		}
		_ = s.conn.SetDeadline(time.Time{})
		return s.stanzaLoop()
	}
}

func (s *Session) openStream(domain string) error {
	from := domain
	if from == "" {
		from = s.srv.domain
	}
	return s.writeString(fmt.Sprintf(
		`<?xml version='1.0'?><stream:stream xmlns='jabber:client' xmlns:stream='http://etherx.jabber.org/streams' from='%s' id='%s' version='1.0'>`,
		xmlEscape(from), s.streamID,
	))
}

func (s *Session) writeFeatures() error {
	var b strings.Builder
	b.WriteString(`<stream:features>`)
	if !s.tlsDone && s.tlsCfg != nil {
		b.WriteString(`<starttls xmlns='urn:ietf:params:xml:ns:xmpp-tls'>`)
		if s.srv.cfg.XMPP.RequireTLS {
			b.WriteString(`<required/>`)
		}
		b.WriteString(`</starttls>`)
	}
	canAuth := s.tlsDone || !s.srv.cfg.XMPP.RequireTLS || s.tlsCfg == nil
	if canAuth && !s.authed {
		b.WriteString(`<mechanisms xmlns='urn:ietf:params:xml:ns:xmpp-sasl'><mechanism>PLAIN</mechanism><mechanism>X-OAUTH2</mechanism></mechanisms>`)
	}
	if canAuth && s.authed && !s.bound {
		b.WriteString(`<bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'/><session xmlns='urn:ietf:params:xml:ns:xmpp-session'/>`)
	}
	b.WriteString(`</stream:features>`)
	return s.writeString(b.String())
}

func (s *Session) negotiate(domain string) error {
	for !s.bound {
		tok, err := s.dec.Token()
		if err != nil {
			return err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch {
		case se.Name.Local == "starttls":
			_ = s.dec.Skip()
			if s.tlsCfg == nil {
				_ = s.writeString(`<failure xmlns='urn:ietf:params:xml:ns:xmpp-tls'/>`)
				return fmt.Errorf("tls not configured")
			}
			if err := s.writeString(`<proceed xmlns='urn:ietf:params:xml:ns:xmpp-tls'/>`); err != nil {
				return err
			}
			tc := tls.Server(s.conn, s.tlsCfg)
			if err := tc.Handshake(); err != nil {
				return err
			}
			s.conn = tc
			s.br = bufio.NewReader(tc)
			s.bw = bufio.NewWriter(tc)
			s.dec = xml.NewDecoder(s.br)
			s.tlsDone = true
			s.streamID = randomID()
			return s.handleStream()
		case se.Name.Local == "auth":
			mech := attr(se, "mechanism")
			raw, err := readElementText(s.dec, se)
			if err != nil {
				return err
			}
			if err := s.saslAuth(mech, raw); err != nil {
				_ = s.writeString(`<failure xmlns='urn:ietf:params:xml:ns:xmpp-sasl'><not-authorized/></failure>`)
				return err
			}
			if err := s.writeString(`<success xmlns='urn:ietf:params:xml:ns:xmpp-sasl'/>`); err != nil {
				return err
			}
			s.authed = true
			s.streamID = randomID()
			return s.handleStream()
		case se.Name.Local == "iq":
			if err := s.handleBindOrSessionIQ(se, domain); err != nil {
				return err
			}
		default:
			_ = s.dec.Skip()
		}
	}
	return nil
}

func (s *Session) saslAuth(mech, b64 string) error {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return err
	}
	parts := strings.Split(string(data), "\x00")
	if len(parts) < 3 {
		return fmt.Errorf("bad sasl payload")
	}
	user, secret := parts[1], parts[2]
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var u *storage.User
	switch strings.ToUpper(mech) {
	case "PLAIN":
		u, err = s.srv.authn.Authenticate(ctx, user, secret)
	case "X-OAUTH2":
		u, err = s.srv.authn.AuthenticateToken(ctx, user, secret)
	default:
		return fmt.Errorf("unsupported mechanism")
	}
	if err != nil {
		return err
	}
	s.user = u
	s.sessionID = s.srv.authn.TrackSession(u, "XMPP", s.conn.RemoteAddr().String(), "", s.conn.RemoteAddr().String(), true, s.conn.Close)
	s.JID = ParseJID(u.Email)
	return nil
}

func (s *Session) handleBindOrSessionIQ(se xml.StartElement, domain string) error {
	id := attr(se, "id")
	typ := attr(se, "type")
	var resource string
	var kind string // bind | session | other
	depth := 1
	for depth > 0 {
		tok, err := s.dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch {
			case t.Name.Local == "bind":
				kind = "bind"
			case t.Name.Local == "session":
				kind = "session"
			case t.Name.Local == "resource":
				r, err := readElementText(s.dec, t)
				if err != nil {
					return err
				}
				depth--
				resource = strings.TrimSpace(r)
			}
		case xml.EndElement:
			depth--
		}
	}
	if typ != "set" {
		return nil
	}
	if kind == "session" {
		return s.writeString(fmt.Sprintf(`<iq type='result' id='%s'/>`, xmlEscape(id)))
	}
	if resource == "" {
		resource = randomID()[:8]
	}
	s.JID.Resource = resource
	if s.JID.Domain == "" {
		s.JID.Domain = domain
	}
	s.bound = true
	s.srv.hub.Register(s)
	if err := s.writeString(fmt.Sprintf(
		`<iq type='result' id='%s'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'><jid>%s</jid></bind></iq>`,
		xmlEscape(id), xmlEscape(s.JID.Full()),
	)); err != nil {
		return err
	}
	if s.user != nil {
		_ = s.srv.flushOffline(context.Background(), s.user.ID, s.SendRaw)
	}
	return nil
}

func (s *Session) stanzaLoop() error {
	for {
		tok, err := s.dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "message":
				if err := s.handleMessage(t); err != nil {
					return err
				}
			case "presence":
				if err := s.handlePresence(t); err != nil {
					return err
				}
			case "iq":
				if err := s.handleIQ(t); err != nil {
					return err
				}
			default:
				_ = s.dec.Skip()
			}
		case xml.EndElement:
			if t.Name.Local == "stream" {
				_ = s.writeString(`</stream:stream>`)
				return io.EOF
			}
		}
	}
}

func (s *Session) handleMessage(se xml.StartElement) error {
	to := attr(se, "to")
	id := attr(se, "id")
	typ := attr(se, "type")
	inner, err := readInnerXML(s.dec, se)
	if err != nil {
		return err
	}
	if to == "" {
		return nil
	}
	toJID := ParseJID(to)
	stanza := fmt.Sprintf(
		`<message from='%s' to='%s'%s%s>%s</message>`,
		xmlEscape(s.JID.Full()),
		xmlEscape(to),
		attrIf("id", id),
		attrIf("type", typ),
		inner,
	)
	b := []byte(stanza)
	ctx := context.Background()
	delivered := s.srv.hub.Route(to, b)
	if !delivered {
		delivered = s.srv.hub.Route(toJID.Bare(), b)
	}
	if !delivered {
		_ = s.srv.storeOffline(ctx, toJID.Bare(), string(b))
	}
	_ = s.srv.enqueueBotInbox(ctx, toJID.Bare(), s.JID.Full(), string(b), extractBody(inner))
	_ = s.srv.archiveMAM(ctx, s.JID.Bare(), toJID.Bare(), id, string(b))
	_ = s.srv.archiveMAM(ctx, toJID.Bare(), s.JID.Bare(), id, string(b))
	s.srv.notifyWebFromStanza(stanza, toJID.Bare())
	s.fanoutCarbon("sent", stanza)
	return nil
}

func (s *Session) fanoutCarbon(kind, forwardedMessage string) {
	for _, other := range s.srv.hub.Sessions(s.JID.Bare()) {
		if other == s {
			continue
		}
		other.mu.Lock()
		on := other.carbons
		other.mu.Unlock()
		if !on {
			continue
		}
		msg := fmt.Sprintf(
			`<message from='%s' to='%s' type='chat'>`+
				`<%s xmlns='urn:xmpp:carbons:2'><forwarded xmlns='urn:xmpp:forward:0'>%s</forwarded></%s>`+
				`</message>`,
			xmlEscape(s.JID.Bare()), xmlEscape(other.JID.Full()), kind, forwardedMessage, kind,
		)
		other.SendRaw([]byte(msg))
	}
}

func (s *Session) handlePresence(se xml.StartElement) error {
	inner, err := readInnerXML(s.dec, se)
	if err != nil {
		return err
	}
	typ := attr(se, "type")
	to := attr(se, "to")
	from := s.JID.Full()
	if to != "" {
		toJID := ParseJID(to)
		stanza := fmt.Sprintf(`<presence from='%s' to='%s'%s>%s</presence>`,
			xmlEscape(from), xmlEscape(to), attrIf("type", typ), inner)
		if !s.srv.hub.Route(toJID.Bare(), []byte(stanza)) &&
			(typ == "subscribe" || typ == "subscribed" || typ == "unsubscribe" || typ == "unsubscribed") {
			_ = s.srv.storeOffline(context.Background(), toJID.Bare(), stanza)
		}
		// auto-update roster subscription hints for local users
		if s.user != nil && (typ == "subscribed" || typ == "unsubscribed") {
			sub := "to"
			if typ == "unsubscribed" {
				sub = "none"
			}
			_ = s.srv.upsertRoster(context.Background(), s.user.ID, rosterItem{
				JID: toJID.Bare(), Subscription: sub, GroupsJSON: "[]",
			})
		}
		return nil
	}
	stanza := fmt.Sprintf(`<presence from='%s'%s>%s</presence>`, xmlEscape(from), attrIf("type", typ), inner)
	for _, other := range s.srv.hub.Sessions(s.JID.Bare()) {
		if other != s {
			other.SendRaw([]byte(stanza))
		}
	}
	return nil
}
