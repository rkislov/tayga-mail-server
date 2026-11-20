package httpapi

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
)

type odfDetailStyle struct {
	node         *compareNode
	part, family string
}
type odfDetailStyles struct {
	pkg         *comparePackage
	definitions map[string]odfDetailStyle
	used        map[string]string
	memo        map[string]string
	active      map[string]bool
	objects     map[string]bool
	defaults    []*compareNode
}

func newODFDetailStyles(pkg *comparePackage, parts ...string) (*odfDetailStyles, error) {
	styles := &odfDetailStyles{pkg: pkg, definitions: map[string]odfDetailStyle{}, used: map[string]string{}, memo: map[string]string{}, active: map[string]bool{}, objects: map[string]bool{}}
	for _, part := range parts {
		if pkg.files[part] == nil {
			continue
		}
		root, err := pkg.xml(part)
		if err != nil {
			return nil, err
		}
		var visit func(*compareNode)
		visit = func(node *compareNode) {
			if node.name.Local == "default-style" {
				styles.defaults = append(styles.defaults, node)
			}
			if name := node.attr("name"); name != "" {
				family := node.attr("family")
				switch node.name.Local {
				case "font-face":
					family = "font-face"
				case "master-page", "page-layout":
					family = node.name.Local
				case "number-style", "date-style", "time-style", "currency-style", "percentage-style", "boolean-style", "text-style":
					family = "data-style"
				}
				if family != "" {
					styles.definitions[family+"\x00"+name] = odfDetailStyle{node, part, family}
				}
			}
			for _, child := range node.children {
				visit(child)
			}
		}
		visit(root)
	}
	return styles, nil
}
func (s *odfDetailStyles) style(name, family string) (string, error) {
	key := family + "\x00" + name
	definition, ok := s.definitions[key]
	if !ok {
		found := false
		for id, value := range s.definitions {
			if strings.HasSuffix(id, "\x00"+name) {
				if found {
					return "", fmt.Errorf("ambiguous ODF style reference")
				}
				key, definition, found = id, value, true
			}
		}
		if !found {
			return "", fmt.Errorf("ODF appearance style missing: %s", name)
		}
	}
	if value, ok := s.memo[key]; ok {
		return value, nil
	}
	if s.active[key] || len(s.active) > 32 {
		return "", fmt.Errorf("cyclic or deep ODF style inheritance")
	}
	s.active[key] = true
	defer delete(s.active, key)
	canon, err := s.canonical(definition.part, definition.node)
	if err != nil {
		return "", err
	}
	projected := projectCompareNode(definition.node, map[string]bool{"notes": true, "annotation": true, "tracked-changes": true, "p": true, "h": true})
	display, hash, err := canon.describe(projected)
	if err != nil {
		return "", err
	}
	s.memo[key] = hash
	s.used[definition.family+":"+hash] = display
	return hash, nil
}
func (s *odfDetailStyles) canonical(part string, root *compareNode) (*detailCanonical, error) {
	canon, err := newDetailCanonical(s.pkg, part, root)
	if err != nil {
		return nil, err
	}
	canon.resolve = func(node *compareNode, attr xml.Attr) (string, bool, error) {
		local := attr.Name.Local
		if local == "name" && strings.Contains(attr.Name.Space, "style:") {
			return "", true, nil
		}
		family := ""
		switch local {
		case "parent-style-name":
			family = node.attr("family")
		case "master-page-name":
			family = "master-page"
		case "page-layout-name":
			family = "page-layout"
		case "data-style-name":
			family = "data-style"
		case "font-name", "font-name-asian", "font-name-complex":
			family = "font-face"
		case "style-name":
			switch node.name.Local {
			case "table":
				family = "table"
			case "table-cell", "covered-table-cell":
				family = "table-cell"
			case "table-row":
				family = "table-row"
			case "table-column":
				family = "table-column"
			case "page":
				family = "drawing-page"
			case "p", "h":
				family = "paragraph"
			case "span":
				family = "text"
			default:
				family = "graphic"
			}
			if strings.Contains(attr.Name.Space, "presentation:") {
				family = "presentation"
			}
		}
		if family != "" || local == "parent-style-name" {
			if attr.Value == "" {
				return "", true, nil
			}
			hash, e := s.style(attr.Value, family)
			return "style:" + hash, true, e
		}
		if local == "href" && (node.name.Local == "image" || node.name.Local == "background-image" || node.name.Local == "object") {
			uri, e := url.Parse(attr.Value)
			if e != nil {
				return "", false, e
			}
			if uri.IsAbs() || uri.Host != "" {
				return "external: " + attr.Value, true, nil
			}
			target := path.Clean(path.Join(path.Dir(part), uri.Path))
			if strings.HasPrefix(target, "../") || target == ".." || strings.HasPrefix(target, "/") {
				return "", false, fmt.Errorf("ODF appearance reference escapes package")
			}
			if node.name.Local == "object" {
				if s.pkg.files[target] == nil {
					target = path.Join(target, "content.xml")
				}
				if s.objects[target] || len(s.objects) > 12 {
					return "", false, fmt.Errorf("cyclic ODF object dependency")
				}
				s.objects[target] = true
				defer delete(s.objects, target)
				tree, e := s.pkg.xml(target)
				if e != nil {
					return "", false, e
				}
				local, e := newODFDetailStyles(s.pkg, path.Join(path.Dir(target), "styles.xml"), target)
				if e != nil {
					return "", false, e
				}
				local.objects = s.objects
				cc, e := local.canonical(target, tree)
				if e != nil {
					return "", false, e
				}
				_, key, e := cc.describe(projectCompareNode(tree, map[string]bool{"automatic-styles": true, "styles": true}))
				return "chart:" + key, true, e
			}
			hash, e := compareImage(s.pkg, target)
			return "image:" + hash, true, e
		}
		return "", false, nil
	}
	return canon, nil
}
func addODFChartDetails(styles *odfDetailStyles, doc *structuredDocument, part string, root *compareNode, group, identity, section string, index int) error {
	number := 0
	for _, object := range root.descendants("object") {
		uri, e := url.Parse(object.attr("href"))
		if e != nil {
			return e
		}
		if uri.IsAbs() || uri.Host != "" {
			return fmt.Errorf("external chart object unsupported in detailed comparison")
		}
		target := path.Clean(path.Join(path.Dir(part), uri.Path))
		if target == ".." || strings.HasPrefix(target, "../") || strings.HasPrefix(target, "/") {
			return fmt.Errorf("chart object escapes package")
		}
		if styles.pkg.files[target] == nil {
			target = path.Join(target, "content.xml")
		}
		tree, e := styles.pkg.xml(target)
		if e != nil {
			return e
		}
		if tree.child("body").child("chart") == nil {
			return fmt.Errorf("embedded ODF object is not a supported chart")
		}
		local, e := newODFDetailStyles(styles.pkg, path.Join(path.Dir(target), "styles.xml"), target)
		if e != nil {
			return e
		}
		canon, e := local.canonical(target, tree)
		if e != nil {
			return e
		}
		number++
		if e = addCompareDetail(doc, section+"\x00"+identity+"\x00"+strconv.Itoa(number), group, section, strconv.Itoa(number), index, canon, projectCompareNode(tree, map[string]bool{"automatic-styles": true, "styles": true})); e != nil {
			return e
		}
	}
	return nil
}
func readCompareODFDetails(pkg *comparePackage, doc *structuredDocument, extension string) error {
	root, err := pkg.xml("content.xml")
	if err != nil {
		return err
	}
	styles, err := newODFDetailStyles(pkg, "styles.xml", "content.xml")
	if err != nil {
		return err
	}
	canon, err := styles.canonical("content.xml", root)
	if err != nil {
		return err
	}
	if extension == ".odp" {
		for index, page := range root.child("body").child("presentation").all("page") {
			id := page.attr("id")
			if id == "" {
				id = page.attr("name")
			}
			name := page.attr("name")
			if name == "" {
				name = strconv.Itoa(index + 1)
			}
			appearance := projectCompareNode(page, map[string]bool{"notes": true, "annotation": true, "p": true, "h": true, "seq": true, "par": true, "animations": true, "animate": true, "animateMotion": true, "animateColor": true, "animateTransform": true, "set": true, "transitionFilter": true, "audio": true})
			if err = addCompareDetail(doc, "slide-style\x00"+id, name, "slide-style", "", index+1, canon, appearance); err != nil {
				return err
			}
			animation := detailContainer("animation")
			for _, node := range page.children {
				if strings.Contains(node.name.Space, "animation:") || node.name.Local == "animations" {
					animation.children = append(animation.children, node)
					animation.parts = append(animation.parts, node)
				}
			}
			if err = addCompareDetail(doc, "slide-animation\x00"+id, name, "slide-animation", "", index+1, canon, animation); err != nil {
				return err
			}
			if pageStyle := page.attr("style-name"); pageStyle != "" {
				transition, e := compareODFTransition(styles, pageStyle)
				if e != nil {
					return e
				}
				if e = addCompareDetail(doc, "slide-transition\x00"+id, name, "slide-transition", "", index+1, canon, transition); e != nil {
					return e
				}
			}
			if err = addODFChartDetails(styles, doc, "content.xml", page, name, id, "slide-chart", index+1); err != nil {
				return err
			}
		}
	} else {
		for index, sheet := range root.child("body").child("spreadsheet").all("table") {
			name := sheet.attr("name")
			// Keep sizing, merge spans and style bindings; omit cell values/formulas
			// already represented by the basic comparison.
			layout := projectCompareNode(sheet, map[string]bool{"p": true, "h": true, "annotation": true, "shapes": true})
			var strip func(*compareNode)
			strip = func(node *compareNode) {
				attrs := []xml.Attr{}
				for _, attr := range node.attrs {
					switch attr.Name.Local {
					case "value", "value-type", "string-value", "boolean-value", "date-value", "time-value", "formula", "currency":
						continue
					}
					attrs = append(attrs, attr)
				}
				node.attrs = attrs
				for _, child := range node.children {
					strip(child)
				}
			}
			strip(layout)
			if err = addCompareDetail(doc, "sheet-layout\x00"+name, name, "sheet-layout", "", index+1, canon, layout); err != nil {
				return err
			}
			if err = addODFChartDetails(styles, doc, "content.xml", sheet, name, name, "sheet-chart", index+1); err != nil {
				return err
			}
		}
	}
	// Defaults and all styles reached from a visible binding are reported with
	// their definitions, not opaque style names. Unused styles do not add noise.
	for _, definition := range styles.defaults {
		if _, _, err = canon.describe(definition); err != nil {
			return err
		}
	}
	keys := make([]string, 0, len(styles.used))
	for key := range styles.used {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	reports := detailContainer("used-styles")
	for _, key := range keys {
		node := detailContainer(strings.SplitN(key, ":", 2)[0])
		node.parts = []any{styles.used[key]}
		reports.parts = append(reports.parts, node)
		reports.children = append(reports.children, node)
	}
	for _, definition := range styles.defaults {
		reports.parts = append(reports.parts, definition)
		reports.children = append(reports.children, definition)
	}
	return addCompareDetail(doc, "odf-styles", "", "theme", "", 0, canon, reports)
}

func compareODFTransition(styles *odfDetailStyles, name string) (*compareNode, error) {
	root := detailContainer("transition")
	seen := map[string]bool{}
	for name != "" {
		if seen[name] || len(seen) > 32 {
			return nil, fmt.Errorf("cyclic page transition style")
		}
		seen[name] = true
		definition, ok := styles.definitions["drawing-page\x00"+name]
		if !ok {
			return nil, fmt.Errorf("drawing page style missing")
		}
		props := definition.node.child("drawing-page-properties")
		selected := &compareNode{name: xml.Name{Local: "page-transition"}}
		if props != nil {
			for _, attr := range props.attrs {
				local := attr.Name.Local
				if strings.HasPrefix(local, "transition-") || local == "duration" || strings.Contains(attr.Name.Space, "smil") {
					selected.attrs = append(selected.attrs, attr)
				}
			}
		}
		if len(selected.attrs) > 0 {
			root.children = append(root.children, selected)
			root.parts = append(root.parts, selected)
		}
		name = definition.node.attr("parent-style-name")
	}
	return root, nil
}
