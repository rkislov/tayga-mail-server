package httpapi

import (
	"encoding/xml"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func slideCompareAppearance(node *compareNode, canon *detailCanonical) (*compareNode, error) {
	if node == nil {
		return nil, nil
	}
	if node.name.Local == "t" || node.name.Local == "creationId" || node.name.Local == "modId" || node.name.Local == "uniqueId" || node.name.Local == "sldLayoutIdLst" || node.name.Local == "timing" || node.name.Local == "transition" {
		return nil, nil
	}
	copy := &compareNode{name: node.name, attrs: append([]xml.Attr(nil), node.attrs...)}
	if node.name.Local == "p" && strings.Contains(node.name.Space, "drawingml/") {
		type run struct {
			key   string
			count int
			props *compareNode
		}
		runs := []run{}
		for _, child := range node.children {
			if child.name.Local == "r" || child.name.Local == "fld" || child.name.Local == "br" {
				props := child.child("rPr")
				_, key, e := canon.describe(props)
				if e != nil {
					return nil, e
				}
				count := utf8.RuneCountInString(drawingCompareText(child))
				if count == 0 {
					continue
				}
				if len(runs) > 0 && runs[len(runs)-1].key == key {
					runs[len(runs)-1].count += count
				} else {
					runs = append(runs, run{key, count, props})
				}
				continue
			}
			projected, e := slideCompareAppearance(child, canon)
			if e != nil {
				return nil, e
			}
			if projected != nil {
				copy.parts = append(copy.parts, projected)
				copy.children = append(copy.children, projected)
			}
		}
		for _, group := range runs {
			if len(runs) == 1 && group.props == nil {
				continue
			}
			child := detailContainer("text-run", group.props)
			if len(runs) > 1 {
				child.attrs = []xml.Attr{{Name: xml.Name{Local: "characters"}, Value: strconv.Itoa(group.count)}}
			}
			copy.parts = append(copy.parts, child)
			copy.children = append(copy.children, child)
		}
		return copy, nil
	}
	for _, child := range node.children {
		projected, e := slideCompareAppearance(child, canon)
		if e != nil {
			return nil, e
		}
		if projected != nil {
			copy.parts = append(copy.parts, projected)
			copy.children = append(copy.children, projected)
		}
	}
	return copy, nil
}

type inheritedAppearance struct {
	role, part   string
	root         *compareNode
	canonical    *detailCanonical
	key, display string
}

func compareInheritedAppearance(pkg *comparePackage, part string) ([]inheritedAppearance, error) {
	out := []inheritedAppearance{}
	seen := map[string]bool{part: true}
	var visit func(string) error
	visit = func(current string) error {
		links, e := pkg.relationships(current)
		if e != nil {
			return e
		}
		ids := []string{}
		for id := range links {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return links[ids[i]].kind < links[ids[j]].kind })
		for _, id := range ids {
			rel := links[id]
			role := path.Base(rel.kind)
			if role != "slideLayout" && role != "slideMaster" && role != "theme" && role != "themeOverride" {
				continue
			}
			if role == "slideLayout" && current != part {
				continue
			} // Master registration lists are not inheritance.
			if rel.external {
				return fmt.Errorf("external appearance dependency unsupported")
			}
			if seen[rel.target] {
				continue
			}
			seen[rel.target] = true
			if len(seen) > 12 {
				return fmt.Errorf("too many inherited appearance parts")
			}
			root, e := pkg.xml(rel.target)
			if e != nil {
				return e
			}
			canonical, e := newDetailCanonical(pkg, rel.target, root)
			if e != nil {
				return e
			}
			projected, e := slideCompareAppearance(root, canonical)
			if e != nil {
				return e
			}
			display, key, e := canonical.describe(projected)
			if e != nil {
				return e
			}
			out = append(out, inheritedAppearance{role, rel.target, root, canonical, key, display})
			if e = visit(rel.target); e != nil {
				return e
			}
		}
		return nil
	}
	err := visit(part)
	return out, err
}
func readComparePPTXDetails(pkg *comparePackage, doc *structuredDocument) error {
	presentation, err := pkg.xml("ppt/presentation.xml")
	if err != nil {
		return err
	}
	links, err := pkg.relationships("ppt/presentation.xml")
	if err != nil {
		return err
	}
	global, err := newDetailCanonical(pkg, "ppt/presentation.xml", presentation)
	if err != nil {
		return err
	}
	if err = addCompareDetail(doc, "presentation-layout", "", "presentation-layout", "", 0, global, detailContainer("presentation-layout", presentation.child("sldSz"), presentation.child("showPr"))); err != nil {
		return err
	}
	themes := map[string]string{}
	for index, slide := range presentation.child("sldIdLst").all("sldId") {
		part := links[slide.relationshipID()].target
		root, e := pkg.xml(part)
		if e != nil {
			return e
		}
		canon, e := newDetailCanonical(pkg, part, root)
		if e != nil {
			return e
		}
		id := slide.plainAttr("id")
		title := root.child("cSld").attr("name")
		if title == "" {
			title = strconv.Itoa(index + 1)
		}
		appearance, e := slideCompareAppearance(root.child("cSld"), canon)
		if e != nil {
			return e
		}
		appearance = detailContainer("slide-appearance", appearance, root.child("clrMapOvr"))
		inherited, e := compareInheritedAppearance(pkg, part)
		if e != nil {
			return e
		}
		for _, dependency := range inherited {
			themes[dependency.role+":"+dependency.key] = dependency.display
			appearance.attrs = append(appearance.attrs, xml.Attr{Name: xml.Name{Local: dependency.role}, Value: dependency.key})
			for _, feature := range []struct{ node, section string }{{"transition", "slide-transition"}, {"timing", "slide-animation"}} {
				if e = addCompareDetail(doc, feature.section+"\x00"+id+"\x00"+dependency.role, title, feature.section, dependency.role, index+1, dependency.canonical, dependency.root.child(feature.node)); e != nil {
					return e
				}
			}
		}
		if e = addCompareDetail(doc, "slide-style\x00"+id, title, "slide-style", "", index+1, canon, appearance); e != nil {
			return e
		}
		for _, feature := range []struct{ node, section string }{{"transition", "slide-transition"}, {"timing", "slide-animation"}} {
			if e = addCompareDetail(doc, feature.section+"\x00"+id, title, feature.section, "", index+1, canon, root.child(feature.node)); e != nil {
				return e
			}
		}
		if _, e = addCompareCharts(pkg, doc, part, root, title, "slide-chart", index+1, 0, id); e != nil {
			return e
		}
	}
	if len(themes) > 0 {
		ids := make([]string, 0, len(themes))
		for id := range themes {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		var display strings.Builder
		for _, id := range ids {
			display.WriteString(strings.SplitN(id, ":", 2)[0])
			display.WriteByte('\n')
			display.WriteString(themes[id])
			display.WriteString("\n\n")
		}
		content := strings.TrimSpace(display.String())
		sum := detailContainer("themes")
		sum.parts = []any{content}
		if err = addCompareDetail(doc, "presentation-themes", "", "theme", "", 0, global, sum); err != nil {
			return err
		}
	}
	return nil
}
