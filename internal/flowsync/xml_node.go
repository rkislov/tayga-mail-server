// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type protocolNode struct {
	Name     xml.Name
	Attr     []xml.Attr
	Text     string
	Children []*protocolNode
}

func parseProtocolXML(body []byte) (*protocolNode, error) {
	d := xml.NewDecoder(bytes.NewReader(body))
	root := &protocolNode{}
	stack := []*protocolNode{root}
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if len(stack) > 64 || (len(stack) == 1 && len(root.Children) != 0) {
				return nil, fmt.Errorf("invalid XML depth or multiple roots")
			}
			n := &protocolNode{Name: t.Name}
			for _, a := range t.Attr {
				if a.Name.Local != "xmlns" && a.Name.Space != "xmlns" {
					n.Attr = append(n.Attr, a)
				}
			}
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, n)
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 1 && strings.TrimSpace(string(t)) != "" {
				return nil, fmt.Errorf("text outside XML root")
			}
			stack[len(stack)-1].Text += string(t)
		}
	}
	if len(root.Children) != 1 || len(stack) != 1 {
		return nil, fmt.Errorf("missing XML root")
	}
	return root, nil
}
func (n *protocolNode) child(name string) *protocolNode {
	for _, c := range n.Children {
		if c.Name.Local == name {
			return c
		}
	}
	return nil
}
func (n *protocolNode) value(name string) string {
	if c := n.child(name); c != nil {
		return c.Text
	}
	return ""
}
func (n *protocolNode) find(name string) *protocolNode {
	if n.Name.Local == name {
		return n
	}
	for _, c := range n.Children {
		if v := c.find(name); v != nil {
			return v
		}
	}
	return nil
}
func (n *protocolNode) render() string {
	var b bytes.Buffer
	e := xml.NewEncoder(&b)
	n.writeXML(e)
	e.Flush()
	return b.String()
}
func (n *protocolNode) writeXML(e *xml.Encoder) {
	start := xml.StartElement{Name: n.Name, Attr: n.Attr}
	if n.Name.Local != "" {
		e.EncodeToken(start)
	}
	if n.Text != "" {
		e.EncodeToken(xml.CharData(n.Text))
	}
	for _, c := range n.Children {
		c.writeXML(e)
	}
	if n.Name.Local != "" {
		e.EncodeToken(start.End())
	}
}
