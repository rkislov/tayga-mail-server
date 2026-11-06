package flowsync

import (
	"encoding/xml"
	"strings"
)

// A client restarting after a crash may retry the exact request whose cached
// response predates the fix. Repair that replay too, preserving its sync token.
func repairEWSTimeZoneNames(response string) string {
	if !strings.Contains(response, "TimeZone") {
		return response
	}
	doc, err := parseProtocolXML([]byte(response))
	if err != nil {
		return response
	}
	changed := false
	var visit func(*protocolNode)
	visit = func(n *protocolNode) {
		if n.Name.Local == "StartTimeZone" || n.Name.Local == "EndTimeZone" {
			if nodeAttr(n, "Name") == "" {
				id := nodeAttr(n, "Id")
				if id != "" {
					found := false
					for i := range n.Attr {
						if n.Attr[i].Name.Local == "Name" {
							n.Attr[i].Value = id
							found = true
						}
					}
					if !found {
						n.Attr = append(n.Attr, xml.Attr{Name: xml.Name{Local: "Name"}, Value: id})
					}
					changed = true
				}
			}
		}
		for _, c := range n.Children {
			visit(c)
		}
	}
	visit(doc)
	if changed {
		return doc.render()
	}
	return response
}
