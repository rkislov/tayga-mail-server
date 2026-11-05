package flowsync

// Repair only the representation of calendar dates in an already accepted
// response. Preserve the SyncKey, IDs, RSVP results and queued client writes.
func repairCalendarDateReplay(response string) (string, error) {
	doc, err := parseProtocolXML([]byte(response))
	if err != nil {
		return "", err
	}
	changed := false
	var visit func(*protocolNode)
	visit = func(n *protocolNode) {
		if n.Name.Space == "Calendar:" {
			switch n.Name.Local {
			case "Until", "StartTime", "EndTime", "ExceptionStartTime", "DTStamp":
				if t := parseASTime(n.Text); t != nil {
					if value := formatCalendarTime(t); value != n.Text {
						n.Text = value
						changed = true
					}
				}
			}
		}
		for _, child := range n.Children {
			visit(child)
		}
	}
	visit(doc)
	if !changed {
		return response, nil
	}
	return doc.render(), nil
}
