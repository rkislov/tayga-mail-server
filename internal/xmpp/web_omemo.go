package xmpp

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const legacyOMEMO = "eu.siacs.conversations.axolotl"

type OMEMOItem struct {
	ID  string `json:"id"`
	XML string `json:"xml"`
}

func legacyOMEMONode(node string) bool {
	if node == legacyOMEMO+".devicelist" {
		return true
	}
	if !strings.HasPrefix(node, legacyOMEMO+".bundles:") {
		return false
	}
	n, e := strconv.ParseUint(strings.TrimPrefix(node, legacyOMEMO+".bundles:"), 10, 31)
	return e == nil && n > 0
}
func (s *Server) ReadWebOMEMO(ctx context.Context, owner, node string) ([]OMEMOItem, error) {
	if !legacyOMEMONode(node) {
		return nil, fmt.Errorf("unsupported OMEMO node")
	}
	items, err := s.getPEPItems(ctx, ParseJID(owner).Bare(), node)
	if err != nil {
		return nil, err
	}
	out := []OMEMOItem{}
	for _, item := range items {
		out = append(out, OMEMOItem{ID: item.ItemID, XML: item.Payload})
	}
	return out, nil
}
func (s *Server) PublishWebOMEMO(ctx context.Context, owner, node, payload string) error {
	if !legacyOMEMONode(node) || len(payload) > 128*1024 {
		return fmt.Errorf("invalid OMEMO publication")
	}
	var root struct{ XMLName xml.Name }
	if xml.Unmarshal([]byte(payload), &root) != nil || root.XMLName.Space != legacyOMEMO {
		return fmt.Errorf("invalid OMEMO XML")
	}
	expected := "bundle"
	if node == legacyOMEMO+".devicelist" {
		expected = "list"
	}
	if root.XMLName.Local != expected {
		return fmt.Errorf("invalid OMEMO element")
	}
	user, err := s.store.GetUserByEmail(ctx, owner)
	if err != nil || !user.Enabled {
		return fmt.Errorf("owner unavailable")
	}
	if err := s.publishPEP(ctx, owner, node, "current", payload); err != nil {
		return err
	}
	session := &Session{srv: s, user: user, JID: ParseJID(owner + "/web")}
	session.notifyPEP(ctx, owner, node, `<item id="current">`+payload+`</item>`)
	return nil
}

// Extract only the encrypted element, never arbitrary client-controlled stanza routing.
func encryptedElement(stanza string) string {
	dec := xml.NewDecoder(strings.NewReader(stanza))
	depth := 0
	for {
		token, err := dec.Token()
		if err != nil {
			return ""
		}
		switch token := token.(type) {
		case xml.StartElement:
			if depth == 1 && token.Name.Local == "encrypted" && (token.Name.Space == legacyOMEMO || token.Name.Space == "urn:xmpp:omemo:2") {
				inner, err := readInnerXML(dec, token)
				if err != nil {
					return ""
				}
				return `<encrypted xmlns="` + token.Name.Space + `">` + inner + `</encrypted>`
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}

}

type encryptedWire struct {
	XMLName xml.Name `xml:"eu.siacs.conversations.axolotl encrypted"`
	Header  struct {
		SID  uint32 `xml:"sid,attr"`
		Keys []struct {
			RID    uint32 `xml:"rid,attr"`
			PreKey string `xml:"prekey,attr,omitempty"`
			Value  string `xml:",chardata"`
		} `xml:"key"`
		IV string `xml:"iv"`
	} `xml:"header"`
	Payload string `xml:"payload,omitempty"`
}

func validateWebCiphertext(raw string) (string, error) {
	if len(raw) > 256*1024 {
		return "", fmt.Errorf("encrypted message too large")
	}
	var wire encryptedWire
	if err := xml.Unmarshal([]byte(raw), &wire); err != nil {
		return "", fmt.Errorf("invalid encrypted message")
	}
	valid64 := func(s string, min, max int) bool {
		b, e := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
		return e == nil && len(b) >= min && len(b) <= max
	}
	if wire.Header.SID == 0 || wire.Header.SID > 2147483647 || len(wire.Header.Keys) == 0 || len(wire.Header.Keys) > 200 || !valid64(wire.Header.IV, 12, 16) {
		return "", fmt.Errorf("invalid encrypted header")
	}
	for _, key := range wire.Header.Keys {
		if key.RID == 0 || key.RID > 2147483647 || !valid64(key.Value, 1, 8192) || (key.PreKey != "" && key.PreKey != "true" && key.PreKey != "false" && key.PreKey != "1" && key.PreKey != "0") {
			return "", fmt.Errorf("invalid recipient key")
		}
	}
	if wire.Payload != "" && !valid64(wire.Payload, 1, 131072) {
		return "", fmt.Errorf("invalid payload")
	}
	b, err := xml.Marshal(wire)
	return string(b), err
}
func (s *Server) SendWebOMEMO(ctx context.Context, from, to, raw string) (ChatEvent, error) {
	peer := ParseJID(to)
	if peer.Local == "" || peer.Domain == "" || s.isWebRoom(to) {
		return ChatEvent{}, fmt.Errorf("OMEMO requires a direct contact")
	}
	// Only local accounts can be routed by this server (no S2S).
	user, err := s.store.GetUserByEmail(ctx, peer.Bare())
	if err != nil || !user.Enabled {
		return ChatEvent{}, fmt.Errorf("recipient unavailable")
	}
	cipher, err := validateWebCiphertext(raw)
	if err != nil {
		return ChatEvent{}, err
	}
	id := randomID()
	fromFull := ParseJID(from).Bare() + "/web"
	stanza := fmt.Sprintf(`<message xmlns="jabber:client" from="%s" to="%s" id="%s" type="chat">%s<store xmlns="urn:xmpp:hints"/><encryption xmlns="urn:xmpp:eme:0" namespace="%s" name="OMEMO"/></message>`, xmlEscape(fromFull), xmlEscape(peer.Bare()), id, cipher, legacyOMEMO)
	stanza, id, err = s.prepareArchivedMessage(ctx, stanza)
	if err != nil {
		return ChatEvent{}, err
	}
	if !s.hub.Route(peer.Bare(), []byte(stanza)) {
		if err = s.storeOffline(ctx, peer.Bare(), stanza); err != nil {
			return ChatEvent{}, err
		}
	}
	if ParseJID(from).Bare() != peer.Bare() {
		(&Session{srv: s, JID: ParseJID(fromFull)}).fanoutCarbon("sent", stanza)
	}
	ev := ChatEvent{Type: "message", ID: id, From: fromFull, To: peer.Bare(), Encrypted: true, OMEMO: cipher, At: time.Now().UTC()}
	s.hub.pushWeb(from, ev)
	if from != peer.Bare() {
		s.hub.pushWeb(peer.Bare(), ev)
	}
	return ev, nil
}
