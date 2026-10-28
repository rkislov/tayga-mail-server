package flowsync

import "strings"

type ewsMailShape struct {
	idOnly, mime bool
	fields       map[string]bool
	extended     map[string]bool
}

func parseEWSMailShape(request string) (ewsMailShape, error) {
	s := ewsMailShape{fields: map[string]bool{}, extended: map[string]bool{}}
	doc, err := parseProtocolXML([]byte(request))
	if err != nil {
		return s, err
	}
	shape := doc.find("ItemShape")
	if shape == nil {
		return s, nil // preserve the legacy default shape
	}
	s.idOnly = shape.value("BaseShape") == "IdOnly"
	s.mime = strings.EqualFold(shape.value("IncludeMimeContent"), "true") || shape.value("IncludeMimeContent") == "1"
	if properties := shape.child("AdditionalProperties"); properties != nil {
		for _, field := range properties.Children {
			switch field.Name.Local {
			case "FieldURI":
				uri := nodeAttr(field, "FieldURI")
				if uri == "item:MimeContent" {
					s.mime = true
				} else {
					s.fields[uri] = true
				}
			case "ExtendedFieldURI":
				s.extended[extendedKey(field)] = true
			}
		}
	}
	return s, nil
}

func (s ewsMailShape) wants(field string) bool { return !s.idOnly || s.fields[field] }

func (s ewsMailShape) project(response string) (string, error) {
	if !s.idOnly {
		return response, nil
	}
	doc, err := parseProtocolXML([]byte(response))
	if err != nil {
		return "", err
	}
	message := doc.find("Message")
	children := message.Children[:0]
	for _, child := range message.Children {
		name := child.Name.Local
		keep := name == "ItemId" || (name == "MimeContent" && s.mime)
		if name == "ExtendedProperty" {
			keep = s.extended[extendedKey(child.child("ExtendedFieldURI"))]
		} else {
			keep = keep || s.fields["item:"+name] || s.fields["message:"+name]
		}
		if keep {
			children = append(children, child)
		}
	}
	message.Children = children
	return doc.render(), nil
}
