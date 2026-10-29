package xmpp

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"errors"
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
	mucResume []mucResumeState // protected by mu; captured on disconnect
	csi       clientState
	sessionID string
	srv       *Server
	log       *slog.Logger
	conn      net.Conn
	done      chan struct{} // closed after serve has finished cleanup and retained SM state
	br        *bufio.Reader
	bw        *bufio.Writer
	dec       *xml.Decoder
	tlsCfg    *tls.Config

	sm           streamManagement
	mu           sync.Mutex
	closed       bool
	authed       bool
	bound        bool
	tlsDone      bool
	carbons      bool
	lastPresence *ChatPresence
	pepInterest  map[string]bool // per-resource +notify features, protected by mu
	pepDiscoID   string
	pepDiscoNode string
	pepCaps      string
	pepDiscoAt   time.Time
	user         *storage.User
	JID          JID
	streamID     string
	streamDomain string
}

func newSession(srv *Server, c net.Conn, alreadyTLS bool) *Session {
	br := bufio.NewReader(c)
	bw := bufio.NewWriter(c)
	return &Session{
		srv:      srv,
		log:      srv.log,
		conn:     c,
		done:     make(chan struct{}),
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
		s.srv.recordPresence(context.Background(), s.JID.Full(), "offline", "")
		s.srv.hub.Unregister(s)
		s.leaveMUC()
	}
	_ = s.conn.Close()
}

func (s *Session) SendRaw(b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.redundantPresence(string(b)) {
		return
	}
	if err := s.writeLocked(string(b)); err != nil && s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *Session) touchSession() {
	if s.srv != nil && s.srv.authn != nil {
		label := "XMPP"
		if s.JID.Resource != "" {
			label += " · " + s.JID.Resource
		}
		s.srv.authn.TouchSession(s.sessionID, label)
	}
}

func (s *Session) disconnectAdmin() error {
	// A deliberate disconnect must not leave a resumable stream behind.
	s.mu.Lock()
	s.sm.resumable = false
	s.mu.Unlock()
	s.Close()
	return nil
}

func (s *Session) writeString(str string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return net.ErrClosed
	}
	return s.writeLocked(str)
}

func (s *Session) serve() {
	defer close(s.done)
	defer s.Close()
	_ = s.conn.SetDeadline(time.Now().Add(5 * time.Minute))
	err := s.handleStream()
	var netErr net.Error
	var xmlErr *xml.SyntaxError
	// encoding/xml wraps EOF inside an open <stream:stream> as a SyntaxError.
	// A mobile transport loss is resumable; malformed XML and a clean stream
	// close are not (the latter explicitly disables resumption in stanzaLoop).
	disconnected := errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) || errors.As(err, &netErr) || (errors.As(err, &xmlErr) && xmlErr.Msg == "unexpected EOF")
	if disconnected {
		s.retainDisconnectedStream()
	}
	if err != nil && err != io.EOF {
		s.log.Info("xmpp session ended", "err", err, "jid", s.JID.Full(), "authenticated", s.authed, "bound", s.bound,
			"peer", s.conn.RemoteAddr().String(), "local", s.conn.LocalAddr().String(), "tls", s.tlsDone)
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
		s.streamDomain = strings.ToLower(strings.TrimSpace(domain))
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
	canAuth := s.tlsDone || !s.srv.cfg.XMPP.RequireTLS
	if canAuth && !s.authed {
		b.WriteString(`<mechanisms xmlns='urn:ietf:params:xml:ns:xmpp-sasl'><mechanism>PLAIN</mechanism><mechanism>X-OAUTH2</mechanism></mechanisms>`)
	}
	if canAuth && s.authed && !s.bound {
		b.WriteString(`<sm xmlns="urn:xmpp:sm:3"/><csi xmlns="urn:xmpp:csi:0"/>`)
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
		case se.Name.Space == smNamespace:
			if err := s.handleStreamManagement(se); err != nil {
				return err
			}
		case se.Name.Local == "starttls" && se.Name.Space == "urn:ietf:params:xml:ns:xmpp-tls":
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
		case se.Name.Local == "auth" && se.Name.Space == "urn:ietf:params:xml:ns:xmpp-sasl":
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
		case se.Name.Local == "iq" && (se.Name.Space == "jabber:client" || se.Name.Space == ""):
			if err := s.handleBindOrSessionIQ(se, domain); err != nil {
				return err
			}
		default:
			_ = s.dec.Skip()
		}
	}
	return nil
}

func (s *Session) saslAuth(mech, b64 string) (authErr error) {
	defer func() {
		if authErr != nil {
			s.log.Warn("xmpp authentication failed", "mechanism", mech, "domain", s.streamDomain, "peer", s.conn.RemoteAddr().String(), "reason", "credentials or SASL policy rejected")
		}
	}()
	if s.srv.cfg.XMPP.RequireTLS && !s.tlsDone {
		return fmt.Errorf("TLS required")
	}
	if s.authed {
		return fmt.Errorf("already authenticated")
	}

	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return err
	}
	parts := strings.Split(string(data), "\x00")
	if len(parts) != 3 {
		return fmt.Errorf("bad sasl payload")
	}
	user, secret := parts[1], parts[2]
	if user == "" {
		return fmt.Errorf("empty authentication identity")
	}
	if !strings.Contains(user, "@") {
		if s.streamDomain == "" {
			return fmt.Errorf("missing stream domain")
		}
		user += "@" + s.streamDomain
	}
	jid, jidErr := NormalizeJID(user)
	if jidErr != nil {
		return jidErr
	}
	if jid.Local == "" || jid.Domain == "" || jid.Resource != "" {
		return fmt.Errorf("invalid authentication identity")
	}
	user = jid.Bare()
	if parts[0] != "" && ParseJID(parts[0]).Bare() != user {
		return fmt.Errorf("authorization identity mismatch")
	}

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
	s.sessionID = s.srv.authn.TrackSession(u, "XMPP", s.conn.RemoteAddr().String(), "XMPP", s.streamID, true, s.disconnectAdmin)
	s.JID = ParseJID(u.Email)
	s.log.Info("xmpp authentication succeeded", "user_id", u.ID, "domain", s.streamDomain, "peer", s.conn.RemoteAddr().String())
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
			case t.Name.Local == "bind" && t.Name.Space == "urn:ietf:params:xml:ns:xmpp-bind":
				kind = "bind"
			case t.Name.Local == "session" && t.Name.Space == "urn:ietf:params:xml:ns:xmpp-session":
				kind = "session"
			case t.Name.Local == "resource":
				r, err := readElementText(s.dec, t)
				if err != nil {
					return err
				}
				depth--
				resource = r
			}
		case xml.EndElement:
			depth--
		}
	}
	if typ != "set" {
		return nil
	}
	if !s.authed || s.user == nil {
		return s.writeString(iqAuthError(id))
	}
	if kind != "bind" && kind != "session" {
		return s.writeString(iqBadRequest(id))
	}
	if kind == "session" {
		return s.writeString(fmt.Sprintf(`<iq type='result' id='%s'/>`, xmlEscape(id)))
	}
	if resource == "" {
		resource = randomID()[:8]
	}
	normalized, err := NormalizeJID(s.JID.Bare() + "/" + resource)
	if err != nil {
		return s.writeString(iqBadRequest(id))
	}
	s.JID.Resource = normalized.Resource
	if s.JID.Domain == "" {
		s.JID.Domain = domain
	}
	s.log.Info("xmpp resource bound", "jid", s.JID.Full())
	s.bound = true
	s.touchSession()
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
			s.touchSession()
			if t.Name.Space == csiNamespace {
				if err := s.handleClientState(t); err != nil {
					return err
				}
				continue
			}
			if t.Name.Space == smNamespace {
				if err := s.handleStreamManagement(t); err != nil {
					return err
				}
				continue
			}
			if t.Name.Space != "" && t.Name.Space != "jabber:client" {
				if err := s.dec.Skip(); err != nil {
					return err
				}
				continue
			}
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
			if t.Name.Local == "message" || t.Name.Local == "presence" || t.Name.Local == "iq" {
				s.mu.Lock()
				if s.sm.enabled {
					s.sm.received++
				}
				s.mu.Unlock()
			}
		case xml.EndElement:
			if t.Name.Local == "stream" {
				s.mu.Lock()
				s.sm.resumable = false
				s.mu.Unlock()
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
	if err := s.srv.checkOMEMOSender(context.Background(), s.JID.Bare(), encryptedElement("<message>"+inner+"</message>")); err != nil {
		return s.writeString(fmt.Sprintf(`<message from='%s' to='%s'%s type='error'><error type='cancel'><not-allowed xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/><text xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'>OMEMO key unavailable</text></error></message>`, xmlEscape(to), xmlEscape(s.JID.Full()), attrIf("id", id)))
	}
	if s.srv.isWebRoom(to) {
		return s.handleMUCMessage(to, typ, id, inner)
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
	ctx := context.Background()
	stanza, _, err = s.srv.prepareArchivedMessage(ctx, stanza)
	if err != nil {
		s.log.Error("xmpp message archive failed", "error", err)
		return s.writeString(fmt.Sprintf(`<message from='%s' to='%s'%s type='error'><error type='wait'><internal-server-error xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></message>`, xmlEscape(to), xmlEscape(s.JID.Full()), attrIf("id", id)))
	}
	b := []byte(stanza)
	delivered := s.srv.hub.Route(to, b)
	if !delivered {
		delivered = s.srv.hub.Route(toJID.Bare(), b)
	}
	if !delivered {
		_ = s.srv.storeOffline(ctx, toJID.Bare(), string(b))
	}
	_ = s.srv.enqueueBotInbox(ctx, toJID.Bare(), s.JID.Full(), string(b), extractBody(inner))
	s.srv.notifyWebFromStanza(stanza, toJID.Bare())
	s.fanoutCarbon("sent", stanza)
	return nil
}

func (s *Session) fanoutCarbon(kind, forwardedMessage string) {
	s.srv.hub.fanoutCarbon(s.JID, kind, forwardedMessage)
}

func (s *Session) handlePresence(se xml.StartElement) error {
	inner, err := readInnerXML(s.dec, se)
	if err != nil {
		return err
	}
	typ := attr(se, "type")
	to := attr(se, "to")
	from := s.JID.Full()
	if s.srv.isWebRoom(to) {
		return s.handleMUCPresence(to, typ, inner)
	}
	if to != "" {
		toJID, err := NormalizeJID(to)
		if err != nil {
			return nil
		}
		if typ == "probe" {
			s.srv.currentPresence(context.Background(), toJID, s.JID, s.SendRaw)
			return nil
		}
		stanza := fmt.Sprintf(`<presence from='%s' to='%s'%s>%s</presence>`,
			xmlEscape(from), xmlEscape(to), attrIf("type", typ), inner)
		if !s.srv.hub.Route(toJID.Full(), []byte(stanza)) &&
			(typ == "subscribe" || typ == "subscribed" || typ == "unsubscribe" || typ == "unsubscribed") {
			_ = s.srv.storeOffline(context.Background(), toJID.Bare(), stanza)
		}
		if s.user != nil {
			ctx := context.Background()
			switch typ {
			case "subscribe":
				_ = s.srv.pendingSubscription(ctx, toJID.Bare(), s.JID.Bare())
			case "subscribed":
				s.srv.clearPresenceRequest(ctx, s.JID.Bare(), toJID.Bare())
				_ = s.srv.changeSubscription(ctx, s.JID.Bare(), toJID.Bare(), false, true)
				_ = s.srv.changeSubscription(ctx, toJID.Bare(), s.JID.Bare(), true, true)
			case "unsubscribed":
				s.srv.clearPresenceRequest(ctx, s.JID.Bare(), toJID.Bare())
				_ = s.srv.changeSubscription(ctx, s.JID.Bare(), toJID.Bare(), false, false)
				_ = s.srv.changeSubscription(ctx, toJID.Bare(), s.JID.Bare(), true, false)
			case "unsubscribe":
				_ = s.srv.changeSubscription(ctx, s.JID.Bare(), toJID.Bare(), true, false)
				_ = s.srv.changeSubscription(ctx, toJID.Bare(), s.JID.Bare(), false, false)
			}
		}

		return nil
	}
	if typ == "" || typ == "unavailable" {
		state := extractPresenceShow(inner)
		if typ == "unavailable" {
			state = "offline"
		}
		var payload struct {
			Status string `xml:"status"`
		}
		_ = xml.Unmarshal([]byte("<presence>"+inner+"</presence>"), &payload)
		s.mu.Lock()
		initial := s.lastPresence == nil || s.lastPresence.State == "offline"
		s.lastPresence = &ChatPresence{State: state, Status: payload.Status}
		s.mu.Unlock()
		s.srv.recordPresencePayload(context.Background(), s.JID.Full(), state, payload.Status, inner, attr(se, "id"))
		if initial && typ == "" {
			s.sendRosterPresence(context.Background())
		}
	}
	if typ == "" {
		s.requestPEPFeatures(inner)
	} else if typ == "unavailable" {
		s.mu.Lock()
		s.pepInterest = nil
		s.pepCaps = ""
		s.pepDiscoID = ""
		s.mu.Unlock()
	}
	stanza := fmt.Sprintf(`<presence from='%s'%s>%s</presence>`, xmlEscape(from), attrIf("type", typ), inner)
	for _, other := range s.srv.hub.Sessions(s.JID.Bare()) {
		if other != s {
			other.SendRaw([]byte(stanza))
		}
	}
	return nil
}
