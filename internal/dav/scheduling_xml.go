// SPDX-License-Identifier: Apache-2.0
package dav

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

const calNS = "urn:ietf:params:xml:ns:caldav"

type davXML struct {
	Name     xml.Name
	Attr     []xml.Attr
	Text     string
	Children []*davXML
}

func xn(ns, name string, children ...*davXML) *davXML {
	return &davXML{Name: xml.Name{Space: ns, Local: name}, Children: children}
}
func xt(ns, name, text string) *davXML { n := xn(ns, name); n.Text = text; return n }
func (n *davXML) child(ns, name string) *davXML {
	if n != nil {
		for _, c := range n.Children {
			if c.Name == (xml.Name{Space: ns, Local: name}) {
				return c
			}
		}
	}
	return nil
}
func (n *davXML) attr(name string) string {
	for _, a := range n.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
func parseDAVXML(raw []byte) (*davXML, error) { return parseDAVXMLLimit(raw, 10000) }
func parseDAVXMLLimit(raw []byte, limit int) (*davXML, error) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	count := 0
	var root *davXML
	var stack []*davXML
	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch v := t.(type) {
		case xml.StartElement:
			count++
			if count > limit || len(stack) > 32 {
				return nil, fmt.Errorf("XML limit")
			}
			n := &davXML{Name: v.Name}
			for _, a := range v.Attr {
				if a.Name.Space != "xmlns" && a.Name.Local != "xmlns" {
					n.Attr = append(n.Attr, a)
				}
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("multiple XML roots")
				}
				root = n
			} else {
				p := stack[len(stack)-1]
				p.Children = append(p.Children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("invalid XML")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string(v)
			} else if len(bytes.TrimSpace(v)) > 0 {
				return nil, fmt.Errorf("text outside root")
			}
		case xml.Directive:
			return nil, fmt.Errorf("XML directives unsupported")
		}
	}
	if root == nil || len(stack) > 0 {
		return nil, fmt.Errorf("XML root required")
	}
	return root, nil
}
func (n *davXML) encode(enc *xml.Encoder) error {
	start := xml.StartElement{Name: n.Name, Attr: n.Attr}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if n.Text != "" {
		if err := enc.EncodeToken(xml.CharData(n.Text)); err != nil {
			return err
		}
	}
	for _, c := range n.Children {
		if err := c.encode(enc); err != nil {
			return err
		}
	}
	return enc.EncodeToken(start.End())
}
func writeDAVXML(w http.ResponseWriter, status int, n *davXML) {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	enc := xml.NewEncoder(&b)
	if err := n.encode(enc); err != nil {
		http.Error(w, "XML failed", 500)
		return
	}
	if err := enc.Flush(); err != nil {
		http.Error(w, "XML failed", 500)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Del("Content-Length")
	w.WriteHeader(status)
	_, _ = w.Write(b.Bytes())
}
func schedulingError(w http.ResponseWriter, status int, name string) {
	writeDAVXML(w, status, xn("DAV:", "error", xn(calNS, name)))
}

type davPropertyRequest struct {
	All, Names bool
	Properties []xml.Name
}

func propertyRequest(root *davXML) davPropertyRequest {
	p := davPropertyRequest{All: root == nil || root.child("DAV:", "allprop") != nil, Names: root.child("DAV:", "propname") != nil}
	if n := root.child("DAV:", "prop"); n != nil {
		for _, c := range n.Children {
			p.Properties = append(p.Properties, c.Name)
		}
	}
	return p
}
func propertyResponse(href string, p davPropertyRequest, values map[xml.Name]*davXML) *davXML {
	response := xn("DAV:", "response", xt("DAV:", "href", href))
	ok, missing := xn("DAV:", "prop"), xn("DAV:", "prop")
	names := p.Properties
	if p.All || p.Names {
		names = nil
		for name := range values {
			names = append(names, name)
		}
	}
	for _, name := range names {
		if v := values[name]; v != nil {
			if p.Names {
				v = xn(name.Space, name.Local)
			}
			ok.Children = append(ok.Children, v)
		} else {
			missing.Children = append(missing.Children, xn(name.Space, name.Local))
		}
	}
	for _, v := range []struct {
		n      *davXML
		status string
	}{{ok, "HTTP/1.1 200 OK"}, {missing, "HTTP/1.1 404 Not Found"}} {
		if len(v.n.Children) > 0 {
			response.Children = append(response.Children, xn("DAV:", "propstat", v.n, xt("DAV:", "status", v.status)))
		}
	}
	return response
}
