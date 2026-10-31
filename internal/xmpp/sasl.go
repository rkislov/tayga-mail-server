package xmpp

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/storage"
)

const sasl2NS = "urn:xmpp:sasl:2"

type scramExchange struct {
	namespace, mechanism, firstBare, serverFirst, nonce, gs2 string
	binding                                                  []byte
	user                                                     *storage.User
	keys                                                     []storage.XMPPSCRAMKey
}

// Capture the exact certificate selected for this connection, not whatever
// certificate happens to be current when a reload races with authentication.
func (s *Session) sessionTLSConfig() *tls.Config {
	cfg := s.tlsCfg.Clone()
	original := cfg.GetCertificate
	certificates := cfg.Certificates
	cfg.Certificates = nil
	cfg.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		var cert *tls.Certificate
		var err error
		if original != nil {
			cert, err = original(hello)
		}
		if err != nil {
			return nil, err
		}
		if cert == nil {
			for i := range certificates {
				if hello.SupportsCertificate(&certificates[i]) == nil {
					cert = &certificates[i]
					break
				}
			}
		}
		if cert == nil || len(cert.Certificate) == 0 {
			return nil, errors.New("no TLS certificate")
		}
		s.localCertificate = append([]byte(nil), cert.Certificate[0]...)
		return cert, nil
	}
	return cfg
}

func (s *Session) setStreamFrom(value string) error {
	s.streamFrom = ""
	if value == "" {
		return nil
	}
	j, err := NormalizeJID(value)
	if err != nil || j.Local == "" || j.Resource != "" {
		return errors.New("invalid stream from")
	}
	if j.Domain != s.streamDomain {
		return errors.New("stream from domain mismatch")
	}
	s.streamFrom = j.Bare()
	return nil
}

func (s *Session) authenticationFeatures() string {
	s.authMechanisms = []string{"PLAIN", "X-OAUTH2"}
	s.channelBindings = map[string][]byte{}
	if s.tlsDone && s.streamFrom != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _, _, err := s.srv.authn.XMPPSCRAMCredentials(ctx, s.streamFrom)
		cancel()
		if err == nil {
			s.authMechanisms = append(s.authMechanisms, "SCRAM-SHA-256")
			// Browsers cannot access TLS channel binding. WSS supports non-PLUS SCRAM.
			if s.ws == nil {
				if tc, ok := s.conn.(*tls.Conn); ok {
					state := tc.ConnectionState()
					if data, err := state.ExportKeyingMaterial("EXPORTER-Channel-Binding", []byte{}, 32); err == nil {
						s.channelBindings["tls-exporter"] = data
					}
					if cert, err := x509.ParseCertificate(s.localCertificate); err == nil {
						switch cert.SignatureAlgorithm {
						case x509.SHA384WithRSA, x509.ECDSAWithSHA384, x509.SHA384WithRSAPSS:
							h := sha512.Sum384(cert.Raw)
							s.channelBindings["tls-server-end-point"] = h[:]
						case x509.SHA512WithRSA, x509.ECDSAWithSHA512, x509.SHA512WithRSAPSS:
							h := sha512.Sum512(cert.Raw)
							s.channelBindings["tls-server-end-point"] = h[:]
						case x509.MD5WithRSA, x509.SHA1WithRSA, x509.ECDSAWithSHA1, x509.SHA256WithRSA, x509.SHA256WithRSAPSS, x509.ECDSAWithSHA256:
							h := sha256.Sum256(cert.Raw)
							s.channelBindings["tls-server-end-point"] = h[:]
						}
					}
				}
				if len(s.channelBindings) > 0 {
					s.authMechanisms = append(s.authMechanisms, "SCRAM-SHA-256-PLUS")
				}
			}
		}
	}
	var b strings.Builder
	mechanisms := func() {
		for _, m := range s.authMechanisms {
			b.WriteString("<mechanism>" + m + "</mechanism>")
		}
	}
	b.WriteString("<mechanisms xmlns='" + saslNS + "'>")
	mechanisms()
	b.WriteString("</mechanisms>")
	if s.tlsDone {
		b.WriteString("<authentication xmlns='" + sasl2NS + "'>")
		mechanisms()
		b.WriteString("</authentication>")
	}
	if len(s.channelBindings) > 0 {
		b.WriteString("<sasl-channel-binding xmlns='urn:xmpp:sasl-cb:0'>")
		for _, cb := range s.bindingNames() {
			b.WriteString("<channel-binding type='" + cb + "'/>")
		}
		b.WriteString("</sasl-channel-binding>")
	}
	return b.String()
}

func (s *Session) bindingNames() []string {
	var names []string
	for name := range s.channelBindings {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func scramAdvertisementHash(mechanisms, bindings []string) string {
	ms := append([]string(nil), mechanisms...)
	bs := append([]string(nil), bindings...)
	sort.Strings(ms)
	sort.Strings(bs)
	data := strings.Join(ms, "\x1e")
	if len(bs) > 0 {
		data += "\x1f" + strings.Join(bs, "\x1e")
	}
	h := sha256.Sum256([]byte(data))
	return base64.StdEncoding.EncodeToString(h[:])
}

// Parse a bounded authentication element without reflecting peer XML/secrets
// into errors. SASL2 user-agent metadata is intentionally not published.
func (s *Session) saslPayload(start xml.StartElement) (string, error) {
	var body strings.Builder
	depth, tokens, bytes := 1, 0, 0
	initial := false
	capture := start.Name.Local != "authenticate"
	for depth > 0 {
		token, err := s.dec.Token()
		if err != nil {
			return "", errors.New("invalid SASL XML")
		}
		tokens++
		if tokens > 256 {
			return "", errors.New("SASL element too large")
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if depth > 4 {
				return "", errors.New("invalid SASL nesting")
			}
			if start.Name.Local != "authenticate" {
				return "", errors.New("invalid SASL content")
			}
			if depth == 2 {
				if t.Name.Space != sasl2NS {
					return "", errors.New("unsupported SASL extension")
				}
				switch t.Name.Local {
				case "initial-response":
					if initial {
						return "", errors.New("duplicate initial response")
					}
					initial = true
					capture = true
				case "user-agent":
					capture = false
				default:
					return "", errors.New("unsupported SASL extension")
				}
			}
			if depth > 2 && capture {
				return "", errors.New("invalid initial response")
			}
		case xml.EndElement:
			if depth == 2 {
				capture = false
			}
			depth--
		case xml.CharData:
			bytes += len(t)
			if bytes > 16384 {
				return "", errors.New("SASL payload too large")
			}
			if capture {
				body.Write(t)
			} else if depth == 1 && strings.TrimSpace(string(t)) != "" {
				return "", errors.New("invalid SASL text")
			}
		case xml.Directive, xml.ProcInst:
			return "", errors.New("invalid SASL XML")
		}
	}
	return strings.TrimSpace(body.String()), nil
}

func (s *Session) saslFailure(namespace, condition string) error {
	s.scram = nil
	_ = s.writeString("<failure xmlns='" + namespace + "'><" + condition + "/></failure>")
	return errors.New("SASL authentication rejected")
}

// SASL2 sends new features directly after success. SASL1 requests a stream
// restart from the transport loop. No inline binding/resumption is advertised.
func (s *Session) handleSASL(se xml.StartElement) (restart bool, err error) {
	ns := se.Name.Space
	if ns != saslNS && ns != sasl2NS {
		return false, errors.New("unexpected traffic during SASL")
	}
	if s.authed || (!s.tlsDone && ns == sasl2NS) {
		return false, s.saslFailure(ns, "not-authorized")
	}
	if se.Name.Local == "abort" {
		return false, s.saslFailure(ns, "aborted")
	}
	payload, e := s.saslPayload(se)
	if e != nil {
		return false, s.saslFailure(ns, "malformed-request")
	}
	data, e := base64.StdEncoding.Strict().DecodeString(payload)
	if e != nil || len(data) > 8192 {
		return false, s.saslFailure(ns, "incorrect-encoding")
	}
	if s.scram != nil {
		if se.Name.Local != "response" || ns != s.scram.namespace {
			return false, s.saslFailure(ns, "malformed-request")
		}
		final, e := s.finishSCRAM(string(data))
		if e != nil {
			return false, s.saslFailure(ns, "not-authorized")
		}
		s.scram = nil
		return s.saslSuccess(ns, final)
	}
	expected := "auth"
	if ns == sasl2NS {
		expected = "authenticate"
	}
	if se.Name.Local != expected {
		return false, s.saslFailure(ns, "malformed-request")
	}
	mech := attr(se, "mechanism")
	if !slices.Contains(s.authMechanisms, mech) {
		return false, s.saslFailure(ns, "invalid-mechanism")
	}
	if strings.HasPrefix(mech, "SCRAM-") {
		first, e := s.beginSCRAM(ns, mech, string(data))
		if e != nil {
			return false, s.saslFailure(ns, "not-authorized")
		}
		return false, s.writeString("<challenge xmlns='" + ns + "'>" + base64.StdEncoding.EncodeToString([]byte(first)) + "</challenge>")
	}
	if err := s.saslAuth(mech, payload); err != nil {
		return false, s.saslFailure(ns, "not-authorized")
	}
	return s.saslSuccess(ns, "")
}

func (s *Session) saslSuccess(ns, additional string) (bool, error) {
	s.authed = true
	encoded := base64.StdEncoding.EncodeToString([]byte(additional))
	if ns == saslNS {
		s.streamID = randomID()
		return true, s.writeString("<success xmlns='" + ns + "'>" + encoded + "</success>")
	}
	body := ""
	if additional != "" {
		body = "<additional-data>" + encoded + "</additional-data>"
	}
	if err := s.writeString("<success xmlns='" + ns + "'>" + body + "<authorization-identifier>" + xmlEscape(s.JID.Bare()) + "</authorization-identifier></success>"); err != nil {
		return false, err
	}
	return false, s.writeFeatures()
}

func scramFields(raw string) (map[string]string, []string, error) {
	fields := map[string]string{}
	var order []string
	if len(raw) > 8192 || !utf8.ValidString(raw) {
		return nil, nil, errors.New("invalid SCRAM data")
	}
	for _, part := range strings.Split(raw, ",") {
		if len(part) < 3 || part[1] != '=' || part[0] < 'a' || part[0] > 'z' {
			return nil, nil, errors.New("invalid SCRAM attribute")
		}
		name := part[:1]
		if _, ok := fields[name]; ok || name == "m" {
			return nil, nil, errors.New("duplicate or mandatory SCRAM attribute")
		}
		fields[name] = part[2:]
		order = append(order, name)
	}
	return fields, order, nil
}
func scramName(value string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '=' {
			if i+3 > len(value) {
				return "", errors.New("bad SCRAM name")
			}
			switch value[i : i+3] {
			case "=2C":
				b.WriteByte(',')
			case "=3D":
				b.WriteByte('=')
			default:
				return "", errors.New("bad SCRAM name")
			}
			i += 2
		} else {
			b.WriteByte(value[i])
		}
	}
	return b.String(), nil
}
func (s *Session) authIdentity(value string) (string, error) {
	if !strings.Contains(value, "@") {
		value += "@" + s.streamDomain
	}
	j, err := NormalizeJID(value)
	if err != nil || j.Local == "" || j.Resource != "" || j.Domain != s.streamDomain {
		return "", errors.New("invalid authentication identity")
	}
	if s.streamFrom != "" && s.streamFrom != j.Bare() {
		return "", errors.New("stream identity mismatch")
	}
	return j.Bare(), nil
}
func (s *Session) beginSCRAM(ns, mechanism, raw string) (string, error) {
	header := strings.SplitN(raw, ",", 3)
	if len(header) != 3 {
		return "", errors.New("invalid GS2 header")
	}
	x := &scramExchange{namespace: ns, mechanism: mechanism, gs2: header[0] + "," + header[1] + ",", firstBare: header[2]}
	if strings.HasSuffix(mechanism, "-PLUS") {
		if !strings.HasPrefix(header[0], "p=") {
			return "", errors.New("channel binding required")
		}
		x.binding = s.channelBindings[strings.TrimPrefix(header[0], "p=")]
		if len(x.binding) == 0 {
			return "", errors.New("unsupported binding")
		}
	} else if header[0] != "n" && (header[0] != "y" || len(s.channelBindings) > 0) {
		return "", errors.New("invalid binding flag")
	}
	fields, order, err := scramFields(x.firstBare)
	if err != nil || len(order) < 2 || order[0] != "n" || order[1] != "r" {
		return "", errors.New("invalid SCRAM first message")
	}
	name, err := scramName(fields["n"])
	if err != nil {
		return "", err
	}
	email, err := s.authIdentity(name)
	if err != nil {
		return "", err
	}
	if header[1] != "" {
		if !strings.HasPrefix(header[1], "a=") {
			return "", errors.New("invalid authzid")
		}
		az, e := scramName(header[1][2:])
		if e != nil {
			return "", e
		}
		az, e = s.authIdentity(az)
		if e != nil || az != email {
			return "", errors.New("authzid mismatch")
		}
	}
	nonce := fields["r"]
	if len(nonce) < 8 || len(nonce) > 512 {
		return "", errors.New("invalid nonce")
	}
	for _, c := range nonce {
		if c < 0x21 || c > 0x7e || c == ',' {
			return "", errors.New("invalid nonce")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u, salt, keys, err := s.srv.authn.XMPPSCRAMCredentials(ctx, email)
	if err != nil {
		return "", err
	}
	random := make([]byte, 24)
	if _, err = rand.Read(random); err != nil {
		return "", err
	}
	x.user, x.keys = u, keys
	x.nonce = nonce + base64.RawStdEncoding.EncodeToString(random)
	x.serverFirst = fmt.Sprintf("r=%s,s=%s,i=%d,h=%s", x.nonce, salt, auth.XMPPSCRAMIterations, scramAdvertisementHash(s.authMechanisms, s.bindingNames()))
	s.scram = x
	return x.serverFirst, nil
}
func scramHMAC(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}
func (s *Session) finishSCRAM(raw string) (string, error) {
	x := s.scram
	fields, order, err := scramFields(raw)
	if err != nil || len(order) < 3 || order[0] != "c" || order[1] != "r" || order[len(order)-1] != "p" || fields["r"] != x.nonce {
		return "", errors.New("invalid SCRAM final message")
	}
	cb, err := base64.StdEncoding.Strict().DecodeString(fields["c"])
	if err != nil || subtle.ConstantTimeCompare(cb, append([]byte(x.gs2), x.binding...)) != 1 {
		return "", errors.New("channel binding mismatch")
	}
	proof, err := base64.StdEncoding.Strict().DecodeString(fields["p"])
	if err != nil || len(proof) != 32 {
		return "", errors.New("invalid SCRAM proof")
	}
	message := x.firstBare + "," + x.serverFirst + "," + raw[:strings.LastIndex(raw, ",p=")]
	match := -1
	for i, k := range x.keys {
		stored, _ := base64.StdEncoding.DecodeString(k.StoredKey)
		signature := scramHMAC(stored, message)
		key := make([]byte, 32)
		for j := range key {
			key[j] = proof[j] ^ signature[j]
		}
		hash := sha256.Sum256(key)
		if subtle.ConstantTimeCompare(hash[:], stored) == 1 {
			match = i
		}
	}
	if match < 0 {
		return "", auth.ErrInvalidCredentials
	}
	k := x.keys[match]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.mu.Lock()
	s.user = x.user
	s.credentialID = k.CredentialID
	s.JID = ParseJID(x.user.Email)
	s.sessionID, err = s.srv.authn.TrackSCRAMXMPPSession(ctx, x.user, k, s.conn.RemoteAddr().String(), "XMPP", s.streamID, s.disconnectAdmin)
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	s.log.Info("xmpp authentication succeeded", "mechanism", x.mechanism, "user_id", x.user.ID, "domain", s.streamDomain, "peer", s.conn.RemoteAddr().String())
	server, _ := base64.StdEncoding.DecodeString(k.ServerKey)
	return "v=" + base64.StdEncoding.EncodeToString(scramHMAC(server, message)), nil
}
