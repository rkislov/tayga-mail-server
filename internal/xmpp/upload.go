package xmpp

import (
	"context"
	"encoding/xml"
	"fmt"
)

const UploadNamespace = "urn:xmpp:http:upload:0"
const MaxUploadSize int64 = 10 << 30

type UploadSlot struct {
	Put string `json:"put"`
	Get string `json:"get"`
}
type UploadHandler func(context.Context, string, string, int64, string) (UploadSlot, error)

// SetUploadHandler may be configured while listeners are running.
func (s *Server) SetUploadHandler(h UploadHandler) {
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	s.upload = h
}
func (s *Session) uploadIQ(id, typ, to, inner string) (bool, error) {
	s.srv.uploadMu.RLock()
	handler := s.srv.upload
	s.srv.uploadMu.RUnlock()
	if handler == nil {
		return false, nil
	}
	var root struct{ XMLName xml.Name }
	if xml.Unmarshal([]byte(inner), &root) != nil {
		return false, nil
	}
	jid := "upload." + s.srv.domain
	reply := func(payload string) error {
		return s.writeString(fmt.Sprintf(`<iq type='result' id='%s' from='%s' to='%s'>%s</iq>`, xmlEscape(id), xmlEscape(to), xmlEscape(s.JID.Full()), payload))
	}
	if typ == "get" && root.XMLName == (xml.Name{Space: "http://jabber.org/protocol/disco#items", Local: "query"}) && (to == s.srv.domain || to == s.JID.Domain || to == "") {
		if to == "" {
			to = s.srv.domain
		}
		return true, reply(`<query xmlns='http://jabber.org/protocol/disco#items'><item jid='` + xmlEscape(jid) + `' name='Tayga file upload'/></query>`)
	}
	if to != jid {
		return false, nil
	}
	if typ == "get" && root.XMLName == (xml.Name{Space: "http://jabber.org/protocol/disco#info", Local: "query"}) {
		return true, reply(fmt.Sprintf(`<query xmlns='http://jabber.org/protocol/disco#info'><identity category='store' type='file' name='Tayga file upload'/><feature var='%s'/><x xmlns='jabber:x:data' type='result'><field var='FORM_TYPE' type='hidden'><value>%s</value></field><field var='max-file-size'><value>%d</value></field></x></query>`, UploadNamespace, UploadNamespace, MaxUploadSize))
	}
	var req struct {
		XMLName     xml.Name
		Filename    string `xml:"filename,attr"`
		Size        int64  `xml:"size,attr"`
		ContentType string `xml:"content-type,attr"`
	}
	if typ != "get" || xml.Unmarshal([]byte(inner), &req) != nil || req.XMLName.Space != UploadNamespace || req.XMLName.Local != "request" {
		return true, s.writeString(iqBadRequest(id))
	}
	slot, err := handler(context.Background(), s.user.ID, req.Filename, req.Size, req.ContentType)
	if err != nil {
		return true, s.writeString(fmt.Sprintf(`<iq type='error' id='%s' from='%s' to='%s'>%s<error type='modify'><not-acceptable xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></iq>`, xmlEscape(id), xmlEscape(to), xmlEscape(s.JID.Full()), inner))
	}
	return true, reply(`<slot xmlns='` + UploadNamespace + `'><put url='` + xmlEscape(slot.Put) + `'/><get url='` + xmlEscape(slot.Get) + `'/></slot>`)
}
