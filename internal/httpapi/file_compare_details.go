package httpapi

import (
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Detailed comparisons describe saved document properties, not rendered pixels.
// References are resolved within the bounded ZIP reader; external links are
// labels only. XML prefixes, attribute order and local service IDs are ignored.
type detailCanonical struct {
	pkg       *comparePackage
	part      string
	relations map[string]compareRelationship
	ids       map[string]string
	stack     map[string]bool
	resolve   func(*compareNode, xml.Attr) (string, bool, error)
}

func newDetailCanonical(pkg *comparePackage, part string, root *compareNode) (*detailCanonical, error) {
	rels, err := pkg.relationships(part)
	if err != nil {
		return nil, err
	}
	c := &detailCanonical{pkg: pkg, part: part, relations: rels, ids: map[string]string{}, stack: map[string]bool{part: true}}
	for _, spec := range []struct{ name, attr, prefix string }{{"cNvPr", "id", "shape"}, {"cTn", "id", "time"}, {"axId", "val", "axis"}} {
		count := 0
		nodes := root.descendants(spec.name)
		for _, node := range nodes {
			value := node.plainAttr(spec.attr)
			if value == "" {
				continue
			}
			key := spec.prefix + ":" + value
			if _, ok := c.ids[key]; !ok {
				count++
				c.ids[key] = fmt.Sprintf("%s-%d", spec.prefix, count)
			}
		}
	}
	objectCount := 0
	var collectObjects func(*compareNode)
	collectObjects = func(node *compareNode) {
		if node == nil {
			return
		}
		for _, a := range node.attrs {
			if a.Name.Local == "id" && (a.Name.Space == "http://www.w3.org/XML/1998/namespace" || strings.Contains(a.Name.Space, "drawing:")) {
				key := "object:" + a.Value
				if _, ok := c.ids[key]; !ok {
					objectCount++
					c.ids[key] = fmt.Sprintf("object-%d", objectCount)
				}
			}
		}
		for _, child := range node.children {
			collectObjects(child)
		}
	}
	collectObjects(root)
	return c, nil
}
func (c *detailCanonical) reference(rel compareRelationship) (string, error) {
	if rel.external {
		return "external: " + rel.target, nil
	}
	kind := path.Base(rel.kind)
	if c.stack[rel.target] {
		return "", fmt.Errorf("cyclic document property relationship")
	}
	if hash, ok := c.pkg.detailHashes[rel.target]; ok {
		return kind + ":" + hash, nil
	}
	if len(c.stack) > 12 {
		return "", fmt.Errorf("document property relationships exceed depth limit")
	}
	if strings.HasSuffix(strings.ToLower(rel.target), ".xml") {
		root, err := c.pkg.xml(rel.target)
		if err != nil {
			return "", err
		}
		next, err := newDetailCanonical(c.pkg, rel.target, root)
		if err != nil {
			return "", err
		}
		next.stack = make(map[string]bool, len(c.stack)+1)
		for key, value := range c.stack {
			next.stack[key] = value
		}
		next.stack[rel.target] = true
		// Masters reference their layouts, which reference the master again. Those
		// registration lists do not describe appearance and are not expanded.
		filtered := projectCompareNode(root, map[string]bool{"sldLayoutIdLst": true, "extLst": false, "creationId": true, "modId": true, "uniqueId": true})
		_, key, err := next.describe(filtered)
		if err != nil {
			return "", err
		}
		if c.pkg.detailHashes == nil {
			c.pkg.detailHashes = map[string]string{}
		}
		c.pkg.detailHashes[rel.target] = key
		return kind + ":" + key, nil
	}
	hash, err := compareImage(c.pkg, rel.target)
	if err != nil {
		return "", err
	}
	return kind + ":" + hash, nil
}
func (c *detailCanonical) attr(node *compareNode, a xml.Attr) (string, bool, error) {
	if a.Name.Local == "xmlns" || a.Name.Space == "xmlns" {
		return "", false, nil
	}
	if c.resolve != nil {
		value, handled, err := c.resolve(node, a)
		if handled || err != nil {
			return value, true, err
		}
	}
	value := a.Value
	if node.name.Local == "numFmt" && a.Name.Local == "numFmtId" && node.attr("formatCode") != "" {
		return node.attr("formatCode"), true, nil
	}
	if a.Name.Local == "id" && (a.Name.Space == "http://www.w3.org/XML/1998/namespace" || strings.Contains(a.Name.Space, "drawing:")) {
		if normalized, ok := c.ids["object:"+value]; ok {
			return normalized, true, nil
		}
	}
	if a.Name.Local == "targetElement" || a.Name.Local == "targetNode" {
		if normalized, ok := c.ids["object:"+strings.TrimPrefix(value, "#")]; ok {
			return normalized, true, nil
		}
	}
	if strings.HasSuffix(a.Name.Space, "/relationships") && (a.Name.Local == "id" || a.Name.Local == "embed" || a.Name.Local == "link") {
		rel, ok := c.relations[value]
		if !ok {
			return "", false, fmt.Errorf("document property relationship missing")
		}
		v, err := c.reference(rel)
		return v, true, err
	}
	prefix := ""
	if node.name.Local == "cNvPr" && a.Name.Local == "id" || node.name.Local == "spTgt" && a.Name.Local == "spid" {
		prefix = "shape"
	}
	if node.name.Local == "cTn" && a.Name.Local == "id" || node.name.Local == "tn" && a.Name.Local == "val" {
		prefix = "time"
	}
	if (node.name.Local == "axId" || node.name.Local == "crossAx") && a.Name.Local == "val" {
		prefix = "axis"
	}
	if prefix != "" {
		if normalized, ok := c.ids[prefix+":"+value]; ok {
			value = normalized
		} else {
			return "", false, fmt.Errorf("document property target missing")
		}
	}
	// Boolean lexical variants are equivalent. Do not drop explicit false values:
	// they may override a true value inherited from a parent style.
	switch a.Name.Local {
	case "b", "i", "hidden", "show", "advClick", "autoRev", "applyFont", "applyFill", "applyBorder", "applyAlignment", "applyProtection", "wrapText", "shrinkToFit":
		if value == "1" {
			value = "true"
		}
		if value == "0" {
			value = "false"
		}
	}
	return value, true, nil
}
func (c *detailCanonical) describe(root *compareNode) (string, string, error) {
	if root == nil {
		return "", "", nil
	}
	var key, display strings.Builder
	properties := 0
	var visit func(*compareNode, string) error
	emit := func(label, value, identity string) error {
		properties++
		if properties > 4000 || display.Len()+len(label)+len(value) > 256<<10 {
			return fmt.Errorf("document property report exceeds limit")
		}
		key.WriteString(strconv.Quote(identity + "\x00" + label))
		key.WriteByte('=')
		key.WriteString(strconv.Quote(value))
		key.WriteByte('\n')
		shownLabel, shownValue := detailPropertyDisplay(label, value)
		display.WriteString(shownLabel)
		display.WriteString(" = ")
		display.WriteString(shownValue)
		display.WriteByte('\n')
		return nil
	}
	visit = func(node *compareNode, location string) error {
		if node == nil || node.name.Local == "creationId" || node.name.Local == "modId" || node.name.Local == "uniqueId" {
			return nil
		}
		identity := node.name.Space + "|" + node.name.Local
		if err := emit(location, "", identity); err != nil {
			return err
		}
		attrs := append([]xml.Attr(nil), node.attrs...)
		sort.Slice(attrs, func(i, j int) bool {
			a, b := attrs[i], attrs[j]
			return a.Name.Space+"|"+a.Name.Local < b.Name.Space+"|"+b.Name.Local
		})
		for _, a := range attrs {
			value, keep, err := c.attr(node, a)
			if err != nil {
				return err
			}
			if keep {
				if err = emit(location+"/@"+a.Name.Local, value, a.Name.Space+"|"+a.Name.Local); err != nil {
					return err
				}
			}
		}
		counts := map[string]int{}
		for _, part := range node.parts {
			switch value := part.(type) {
			case string:
				if strings.TrimSpace(value) != "" || node.name.Local == "v" || node.name.Local == "t" || node.name.Local == "p" {
					if err := emit(location+"/text", value, "text"); err != nil {
						return err
					}
				}
			case *compareNode:
				counts[value.name.Local]++
				if err := visit(value, fmt.Sprintf("%s/%s[%d]", location, value.name.Local, counts[value.name.Local])); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(root, root.name.Local); err != nil {
		return "", "", err
	}
	hash := sha256.Sum256([]byte(key.String()))
	return strings.TrimSpace(display.String()), fmt.Sprintf("%x", hash[:]), nil
}

// Display units do not participate in the fingerprint: rounding a coordinate
// for humans must never hide a change in the original saved property.
func detailPropertyDisplay(label, value string) (string, string) {
	if value == "" {
		return label, value
	}
	if strings.HasSuffix(label, "/@sz") && (strings.Contains(label, "rPr[") || strings.Contains(label, "defRPr[") || strings.Contains(label, "endParaRPr[")) {
		if size, e := strconv.ParseFloat(value, 64); e == nil {
			return strings.TrimSuffix(label, "/@sz") + "/@font-size", strconv.FormatFloat(size/100, 'g', -1, 64) + " pt (" + value + ")"
		}
	}
	if strings.Contains(label, "/sz[") && strings.HasSuffix(label, "/@val") {
		return strings.TrimSuffix(label, "/@val") + "/@font-size", value + " pt"
	}
	if strings.HasSuffix(label, "/@typeface") {
		return strings.TrimSuffix(label, "/@typeface") + "/@font-family", value
	}
	if strings.Contains(label, "/srgbClr[") && strings.HasSuffix(label, "/@val") {
		return label, "#" + value
	}
	if strings.HasSuffix(label, "/@rgb") {
		return label, "#" + value
	}
	if strings.HasSuffix(label, "/@dur") {
		if _, e := strconv.ParseUint(value, 10, 64); e == nil {
			return strings.TrimSuffix(label, "/@dur") + "/@duration", value + " ms"
		}
	}
	for _, attr := range []string{"x", "y", "cx", "cy"} {
		if strings.HasSuffix(label, "/@"+attr) && (strings.Contains(label, "/off[") || strings.Contains(label, "/ext[")) {
			if distance, e := strconv.ParseFloat(value, 64); e == nil {
				return label, fmt.Sprintf("%.3f mm (%s EMU)", distance/36000, value)
			}
		}
	}
	return label, value
}
func projectCompareNode(node *compareNode, drop map[string]bool) *compareNode {
	if node == nil || drop[node.name.Local] {
		return nil
	}
	copy := &compareNode{name: node.name, attrs: append([]xml.Attr(nil), node.attrs...)}
	for _, part := range node.parts {
		switch value := part.(type) {
		case string:
			copy.parts = append(copy.parts, value)
		case *compareNode:
			if child := projectCompareNode(value, drop); child != nil {
				copy.children = append(copy.children, child)
				copy.parts = append(copy.parts, child)
			}
		}
	}
	return copy
}
func detailContainer(name string, nodes ...*compareNode) *compareNode {
	root := &compareNode{name: xml.Name{Space: "tayga:comparison", Local: name}}
	for _, node := range nodes {
		if node != nil {
			root.children = append(root.children, node)
			root.parts = append(root.parts, node)
		}
	}
	return root
}
func addCompareDetail(doc *structuredDocument, id, group, section, address string, index int, canonical *detailCanonical, node *compareNode) error {
	if node == nil || len(node.parts) == 0 && len(node.attrs) == 0 {
		return nil
	}
	display, key, err := canonical.describe(node)
	if err != nil {
		return err
	}
	return doc.add("detail\x00"+id, structuredEntry{group, section, address, display, key, fmt.Sprintf("%04d:8:%s:%s", index, section, address), index})
}
func readComparisonDetails(pkg *comparePackage, doc *structuredDocument, extension string) error {
	switch extension {
	case ".xlsx":
		return readCompareXLSXDetails(pkg, doc)
	case ".pptx":
		return readComparePPTXDetails(pkg, doc)
	case ".ods", ".odp":
		return readCompareODFDetails(pkg, doc, extension)
	}
	return nil
}
