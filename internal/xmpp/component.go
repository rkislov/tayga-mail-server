package xmpp

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/tayga/tms/internal/config"
)

type componentSession struct {
	srv      *Server
	cfg      config.XMPPComponentConfig
	domain   string
	secret   string
	conn     net.Conn
	br       *bufio.Reader
	bw       *bufio.Writer
	dec      *xml.Decoder
	streamID string
	hubConn  *ComponentConn
	mu       sync.Mutex
	closed   bool
}

func (s *Server) componentDomain(c config.XMPPComponentConfig) string {
	if c.Domain != "" {
		return strings.ToLower(c.Domain)
	}
	sub := strings.TrimSpace(c.Subdomain)
	if sub == "" {
		sub = strings.TrimSpace(c.Name)
	}
	if sub == "" || s.domain == "" {
		return ""
	}
	return strings.ToLower(sub + "." + s.domain)
}

func (s *Server) componentSecret(domain string) (config.XMPPComponentConfig, bool) {
	want := strings.ToLower(domain)
	for _, c := range s.cfg.XMPP.Components {
		if s.componentDomain(c) == want && c.Secret != "" {
			return c, true
		}
	}
	return config.XMPPComponentConfig{}, false
}

func (s *Server) startComponents(ctx context.Context) error {
	if s.cfg == nil || len(s.cfg.XMPP.Components) == 0 {
		return nil
	}
	addr := s.cfg.XMPP.ComponentListen
	if addr == "" {
		addr = ":5347"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.ln = append(s.ln, ln)
	s.mu.Unlock()
	s.log.Info("xmpp component listener", "addr", addr, "count", len(s.cfg.XMPP.Components))
	for _, c := range s.cfg.XMPP.Components {
		s.log.Info("xmpp component registered", "name", c.Name, "domain", s.componentDomain(c))
	}
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	go s.acceptComponents(ln)
	return nil
}

func (s *Server) acceptComponents(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			continue
		}
		go s.serveComponent(c)
	}
}

func (s *Server) serveComponent(c net.Conn) {
	br := bufio.NewReader(c)
	bw := bufio.NewWriter(c)
	cs := &componentSession{
		srv: s, conn: c, br: br, bw: bw,
		dec: xml.NewDecoder(br), streamID: randomID(),
	}
	defer cs.close()
	_ = c.SetDeadline(time.Now().Add(2 * time.Minute))
	if err := cs.handshake(); err != nil {
		s.log.Debug("xmpp component handshake failed", "err", err)
		return
	}
	_ = c.SetDeadline(time.Time{})
	s.log.Info("xmpp component connected", "domain", cs.domain)
	_ = cs.stanzaLoop()
}

func (cs *componentSession) close() {
	cs.mu.Lock()
	if cs.closed {
		cs.mu.Unlock()
		return
	}
	cs.closed = true
	cs.mu.Unlock()
	if cs.domain != "" && cs.hubConn != nil {
		cs.srv.hub.UnregisterComponent(cs.domain, cs.hubConn)
	}
	_ = cs.conn.Close()
}

func (cs *componentSession) ensureHubConn() *ComponentConn {
	if cs.hubConn == nil {
		cs.hubConn = &ComponentConn{
			Domain: cs.domain,
			Send:   cs.SendRaw,
			Close:  func() { cs.close() },
		}
	}
	return cs.hubConn
}

func (cs *componentSession) writeString(str string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.closed {
		return net.ErrClosed
	}
	if _, err := cs.bw.WriteString(str); err != nil {
		return err
	}
	return cs.bw.Flush()
}

func (cs *componentSession) SendRaw(b []byte) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.closed {
		return
	}
	_, _ = cs.bw.Write(b)
	_ = cs.bw.Flush()
}

func (cs *componentSession) handshake() error {
	for {
		tok, err := cs.dec.Token()
		if err != nil {
			return err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local != "stream" {
			return fmt.Errorf("expected stream")
		}
		to := attr(se, "to")
		cfg, ok := cs.srv.componentSecret(to)
		if !ok {
			_ = cs.writeString(`<stream:error><host-unknown xmlns='urn:ietf:params:xml:ns:xmpp-streams'/></stream:error></stream:stream>`)
			return fmt.Errorf("unknown component domain %q", to)
		}
		cs.cfg = cfg
		cs.domain = cs.srv.componentDomain(cfg)
		cs.secret = cfg.Secret

		if err := cs.writeString(fmt.Sprintf(
			`<?xml version='1.0'?><stream:stream xmlns:stream='http://etherx.jabber.org/streams' xmlns='jabber:component:accept' from='%s' id='%s'>`,
			xmlEscape(cs.domain), cs.streamID,
		)); err != nil {
			return err
		}

		for {
			tok, err := cs.dec.Token()
			if err != nil {
				return err
			}
			se, ok := tok.(xml.StartElement)
			if !ok {
				continue
			}
			if se.Name.Local != "handshake" {
				_ = cs.dec.Skip()
				continue
			}
			got, err := readElementText(cs.dec, se)
			if err != nil {
				return err
			}
			sum := sha1.Sum([]byte(cs.streamID + cs.secret))
			want := hex.EncodeToString(sum[:])
			if !strings.EqualFold(strings.TrimSpace(got), want) {
				_ = cs.writeString(`<stream:error><not-authorized xmlns='urn:ietf:params:xml:ns:xmpp-streams'/></stream:error></stream:stream>`)
				return fmt.Errorf("bad handshake")
			}
			if err := cs.writeString(`<handshake/>`); err != nil {
				return err
			}
			cs.srv.hub.RegisterComponent(cs.ensureHubConn())
			return nil
		}
	}
}

func (cs *componentSession) stanzaLoop() error {
	for {
		tok, err := cs.dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "message", "presence", "iq":
				if err := cs.forwardStanza(t); err != nil {
					return err
				}
			default:
				_ = cs.dec.Skip()
			}
		case xml.EndElement:
			if t.Name.Local == "stream" {
				_ = cs.writeString(`</stream:stream>`)
				return io.EOF
			}
		}
	}
}

func (cs *componentSession) forwardStanza(se xml.StartElement) error {
	ctx := context.Background()
	name := se.Name.Local
	rawFrom := attr(se, "from")
	to := attr(se, "to")
	id := attr(se, "id")
	typ := attr(se, "type")
	inner, err := readInnerXML(cs.dec, se)
	if err != nil {
		return err
	}
	from := rewriteComponentFrom(rawFrom, cs.domain)
	if to == "" {
		return nil
	}
	stanza := fmt.Sprintf(`<%s from='%s' to='%s'%s%s>%s</%s>`,
		name, xmlEscape(from), xmlEscape(to), attrIf("id", id), attrIf("type", typ), inner, name,
	)
	b := []byte(stanza)
	delivered := cs.srv.hub.Route(to, b)
	if name == "message" {
		if !delivered {
			_ = cs.srv.storeOffline(ctx, ParseJID(to).Bare(), string(b))
		}
		_ = cs.srv.enqueueBotInbox(ctx, ParseJID(to).Bare(), from, string(b), extractBody(inner))
		_ = cs.srv.archiveMAM(ctx, ParseJID(from).Bare(), ParseJID(to).Bare(), id, string(b))
		_ = cs.srv.archiveMAM(ctx, ParseJID(to).Bare(), ParseJID(from).Bare(), id, string(b))
		cs.srv.notifyWebFromStanza(stanza, ParseJID(to).Bare())
	}
	return nil
}

func rewriteComponentFrom(from, componentDomain string) string {
	if from == "" {
		return componentDomain
	}
	j := ParseJID(from)
	if j.Domain == componentDomain {
		return j.Full()
	}
	if j.Local == "" {
		return componentDomain
	}
	out := j.Local + "@" + componentDomain
	if j.Resource != "" {
		out += "/" + j.Resource
	}
	return out
}

func extractBody(inner string) string {
	const open = "<body"
	i := strings.Index(strings.ToLower(inner), open)
	if i < 0 {
		return ""
	}
	rest := inner[i:]
	gt := strings.IndexByte(rest, '>')
	if gt < 0 {
		return ""
	}
	rest = rest[gt+1:]
	end := strings.Index(strings.ToLower(rest), "</body>")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// ComponentHandshakeDigest is exported for tests (SHA-1 of streamID+secret).
func ComponentHandshakeDigest(streamID, secret string) string {
	sum := sha1.Sum([]byte(streamID + secret))
	return hex.EncodeToString(sum[:])
}
