package xmpp

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

const framingNS = "urn:ietf:params:xml:ns:xmpp-framing"
const maxWebSocketFrame = 1 << 20
const maxWebSocketConnections = 256

// ServeWebSocket implements RFC 7395. TLS must terminate here: untrusted
// Forwarded/X-Forwarded-Proto headers cannot authorize plaintext credentials.
func (s *Server) ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	if s.cfg == nil || !s.cfg.XMPP.Enabled || !s.cfg.XMPP.WebSocket {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.TLS == nil {
		http.Error(w, "HTTPS required", http.StatusForbidden)
		return
	}
	if r.URL.RawQuery != "" || !webSocketOriginAllowed(r) {
		http.Error(w, "invalid WebSocket request", http.StatusForbidden)
		return
	}
	hasProtocol := false
	for _, protocol := range websocket.Subprotocols(r) {
		hasProtocol = hasProtocol || protocol == "xmpp"
	}
	if !hasProtocol {
		http.Error(w, "xmpp subprotocol required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	if s.closed || s.webSocketCount >= maxWebSocketConnections {
		s.mu.Unlock()
		http.Error(w, "XMPP transport unavailable", http.StatusServiceUnavailable)
		return
	}
	s.webSocketCount++
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.webSocketCount--; s.mu.Unlock() }()
	upgrader := websocket.Upgrader{
		Subprotocols: []string{"xmpp"}, HandshakeTimeout: 5 * time.Second,
		ReadBufferSize: 4096, WriteBufferSize: 4096,
		CheckOrigin: webSocketOriginAllowed,
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ws.SetReadLimit(maxWebSocketFrame)
	session := newSession(s, ws.UnderlyingConn(), true)
	session.ws = ws
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = ws.Close()
		return
	}
	if s.webSockets == nil {
		s.webSockets = make(map[*Session]struct{})
	}
	s.webSockets[session] = struct{}{}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.webSockets, session); s.mu.Unlock() }()
	session.serve()
}

func webSocketOriginAllowed(r *http.Request) bool {
	origins := r.Header.Values("Origin")
	if len(origins) == 0 {
		return true // Native WebSocket clients need not send Origin; SASL is still mandatory.
	}
	if len(origins) != 1 {
		return false
	}
	u, err := url.Parse(origins[0])
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	canonical := func(host string) string { return strings.TrimSuffix(strings.ToLower(host), ":443") }
	return canonical(u.Host) == canonical(r.Host)
}

// decodeWebSocketFrame validates a complete, bounded XML document before any
// handler can mutate state. A message cannot smuggle a second stanza or DTD.
func decodeWebSocketFrame(raw []byte) (xml.StartElement, *xml.Decoder, error) {
	var root xml.StartElement
	if len(raw) == 0 || raw[0] != '<' || len(raw) > maxWebSocketFrame || !utf8.Valid(raw) {
		return root, nil, fmt.Errorf("invalid XML frame")
	}
	d := xml.NewDecoder(bytes.NewReader(raw))
	depth, roots := 0, 0
	declaration := false
	for {
		token, err := d.Token()
		if err == io.EOF && depth == 0 && roots == 1 {
			break
		}
		if err != nil {
			return root, nil, fmt.Errorf("invalid XML frame")
		}
		switch t := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				root = t
			}
			depth++
			if roots > 1 || depth > 64 {
				return root, nil, fmt.Errorf("invalid XML frame")
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && len(bytes.TrimSpace(t)) != 0 {
				return root, nil, fmt.Errorf("invalid XML frame")
			}
		case xml.ProcInst:
			if t.Target != "xml" || roots != 0 || declaration {
				return root, nil, fmt.Errorf("XML processing instructions are not supported")
			}
			declaration = true
		case xml.Directive:
			return root, nil, fmt.Errorf("XML directives are not supported")
		}
	}
	if err := validateWebSocketNamespaces(raw); err != nil {
		return root, nil, err
	}
	d = xml.NewDecoder(bytes.NewReader(raw))
	for {
		token, err := d.Token()
		if err != nil {
			return root, nil, err
		}
		if _, ok := token.(xml.StartElement); ok {
			return root, d, nil
		}
	}
}

func (s *Session) handleWebSocketStream() error {
	// A bounded deadline covers opening, SASL and binding; a bound stream uses
	// XMPP Ping/SM and WebSocket control pings instead of whitespace keepalives.
	_ = s.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	expectOpen, opened := true, false
	for {
		typ, raw, err := s.ws.ReadMessage()
		if err != nil {
			var closeErr *websocket.CloseError
			if errors.As(err, &closeErr) {
				if closeErr.Code == websocket.CloseNormalClosure || closeErr.Code == websocket.CloseGoingAway || closeErr.Code == websocket.CloseAbnormalClosure {
					return io.EOF // No XMPP <close/>: allow the shared SM recovery path.
				}
				return fmt.Errorf("XMPP WebSocket closed with code %d", closeErr.Code) // Do not log peer-supplied close text.
			}
			return err
		}
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()
		if closed {
			return io.EOF
		}
		if typ != websocket.TextMessage {
			return s.webSocketFailure("unsupported-stanza-type", opened)
		}
		start, dec, err := decodeWebSocketFrame(raw)
		if err != nil {
			return s.webSocketFailure("not-well-formed", opened)
		}
		s.dec = dec
		if start.Name.Local == "open" || start.Name.Local == "close" {
			if start.Name.Space != framingNS {
				return s.webSocketFailure("invalid-namespace", opened)
			}
			inner, err := readInnerXML(s.dec, start)
			if err != nil || strings.TrimSpace(inner) != "" {
				return s.webSocketFailure("bad-format", opened)
			}
			if start.Name.Local == "close" {
				s.mu.Lock()
				s.sm.resumable = false
				s.mu.Unlock()
				_ = s.writeString(`<close xmlns='` + framingNS + `'/>`)
				_ = s.ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
				return io.EOF
			}
			if !expectOpen {
				return s.webSocketFailure("policy-violation", opened)
			}
			jid, err := NormalizeJID(attr(start, "to"))
			if err != nil || jid.Local != "" || jid.Resource != "" {
				return s.webSocketFailure("improper-addressing", opened)
			}
			if s.streamDomain != "" && s.streamDomain != jid.Domain {
				return s.webSocketFailure("host-unknown", opened)
			}
			if jid.Domain != s.srv.domain {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err = s.srv.store.GetDomainByName(ctx, jid.Domain)
				cancel()
				if err != nil {
					return s.webSocketFailure("host-unknown", opened)
				}
			}
			if attr(start, "version") != "1.0" {
				return s.webSocketFailure("unsupported-version", opened)
			}
			s.streamDomain = jid.Domain
			if err := s.setStreamFrom(attr(start, "from")); err != nil {
				return s.webSocketFailure("invalid-from", opened)
			}
			if err := s.writeString(fmt.Sprintf(`<open xmlns='%s' from='%s' id='%s' version='1.0' xml:lang='en'/>`, framingNS, xmlEscape(jid.Domain), s.streamID)); err != nil {
				return err
			}
			opened, expectOpen = true, false
			if err := s.writeFeatures(); err != nil {
				return err
			}
			continue
		}
		if expectOpen {
			return s.webSocketFailure("policy-violation", opened)
		}
		if s.scram != nil && start.Name.Space != saslNS && start.Name.Space != sasl2NS {
			return s.webSocketFailure("policy-violation", opened)
		}
		if start.Name.Space == "urn:ietf:params:xml:ns:xmpp-tls" {
			return s.webSocketFailure("policy-violation", opened)
		}
		if s.bound {
			if start.Name.Space != "jabber:client" && start.Name.Space != smNamespace && start.Name.Space != csiNamespace {
				return s.webSocketFailure("invalid-namespace", opened)
			}
			if err := s.handleBoundElement(start); err != nil {
				return err
			}
			continue
		}
		switch {
		case start.Name.Space == saslNS || start.Name.Space == sasl2NS:
			restart, err := s.handleSASL(start)
			if err != nil {
				return s.webSocketFailure("not-authorized", opened)
			}
			if restart {
				expectOpen, opened = true, false
			}

		case start.Name.Space == smNamespace && s.authed:
			if err := s.handleStreamManagement(start); err != nil {
				return err
			}
		case start.Name.Space == "jabber:client" && start.Name.Local == "iq" && s.authed:
			if err := s.handleBindOrSessionIQ(start, s.streamDomain); err != nil {
				return err
			}
		default:
			return s.webSocketFailure("not-authorized", opened)
		}
		if s.bound {
			_ = s.conn.SetDeadline(time.Time{})
		}
	}
}

// encoding/xml accepts undeclared prefixes. RFC 7395 frames have no inherited
// stream namespace context, so validate prefix scopes explicitly.
func validateWebSocketNamespaces(raw []byte) error {
	d := xml.NewDecoder(bytes.NewReader(raw))
	stack := []map[string]string{{"xml": "http://www.w3.org/XML/1998/namespace"}}
	for {
		token, err := d.RawToken()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("invalid XML namespaces")
		}
		switch t := token.(type) {
		case xml.StartElement:
			scope := make(map[string]string, len(stack[len(stack)-1])+2)
			for key, value := range stack[len(stack)-1] {
				scope[key] = value
			}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" {
					if a.Name.Local == "xmlns" || (a.Name.Local == "xml") != (a.Value == "http://www.w3.org/XML/1998/namespace") || a.Value == "" || a.Value == "http://www.w3.org/2000/xmlns/" {
						return fmt.Errorf("invalid XML namespaces")
					}
					scope[a.Name.Local] = a.Value
				} else if a.Name.Space == "" && a.Name.Local == "xmlns" {
					if a.Value == "http://www.w3.org/XML/1998/namespace" || a.Value == "http://www.w3.org/2000/xmlns/" {
						return fmt.Errorf("invalid XML namespaces")
					}
					scope[""] = a.Value
				}
			}
			if t.Name.Space != "" && scope[t.Name.Space] == "" {
				return fmt.Errorf("undeclared XML prefix")
			}
			seen := make(map[xml.Name]bool)
			for _, a := range t.Attr {
				name := a.Name
				if name.Space != "" && name.Space != "xmlns" {
					if scope[name.Space] == "" {
						return fmt.Errorf("undeclared XML prefix")
					}
					name.Space = scope[name.Space]
				}
				if seen[name] {
					return fmt.Errorf("duplicate XML attribute")
				}
				seen[name] = true
			}
			stack = append(stack, scope)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
}

func (s *Session) webSocketFailure(condition string, opened bool) error {
	s.mu.Lock()
	s.sm.resumable = false
	s.mu.Unlock()
	if !opened {
		_ = s.writeString(`<open xmlns='` + framingNS + `' from='` + xmlEscape(s.srv.domain) + `' id='` + s.streamID + `' version='1.0'/>`)
	}
	_ = s.writeString(`<stream:error><` + condition + ` xmlns='urn:ietf:params:xml:ns:xmpp-streams'/></stream:error>`)
	_ = s.writeString(`<close xmlns='` + framingNS + `'/>`)
	_ = s.ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseProtocolError, "XMPP stream error"), time.Now().Add(time.Second))
	return fmt.Errorf("XMPP WebSocket: %s", condition) // No peer XML, credentials or message content.
}

func (s *Session) setWriteDeadline(deadline time.Time) {
	if s.ws != nil {
		_ = s.ws.SetWriteDeadline(deadline)
	} else {
		_ = s.conn.SetWriteDeadline(deadline)
	}
}

func (s *Session) writeElementLocked(raw string) error {
	if s.ws == nil {
		_, err := s.bw.WriteString(raw)
		return err
	}
	// TCP inherits these namespace declarations from stream:stream; every
	// WebSocket element is standalone. Add only missing root declarations.
	d := xml.NewDecoder(strings.NewReader(raw))
	token, err := d.Token()
	start, ok := token.(xml.StartElement)
	if err != nil || !ok {
		return fmt.Errorf("invalid outgoing XMPP WebSocket element")
	}
	hasDefault, hasStream := false, false
	for _, a := range start.Attr {
		hasDefault = hasDefault || (a.Name.Space == "" && a.Name.Local == "xmlns")
		hasStream = hasStream || (a.Name.Space == "xmlns" && a.Name.Local == "stream")
	}
	decl := ""
	if !hasDefault {
		decl += ` xmlns='jabber:client'`
	}
	if !hasStream {
		decl += ` xmlns:stream='` + streamNS + `'`
	}
	end := strings.IndexAny(raw, " />\t\r\n")
	if end < 0 {
		return fmt.Errorf("invalid outgoing XMPP WebSocket element")
	}
	raw = raw[:end] + decl + raw[end:]
	return s.ws.WriteMessage(websocket.TextMessage, []byte(raw))
}

func (s *Session) flushElementsLocked() error {
	if s.ws != nil {
		return nil
	}
	return s.bw.Flush()
}
