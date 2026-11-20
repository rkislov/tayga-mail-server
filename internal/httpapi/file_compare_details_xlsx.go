package httpapi

import (
	"encoding/xml"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

type compareXLSXStyles struct {
	fonts, fills, borders, xfs, parents, dxfs []*compareNode
	formats                                   map[string]string
}

func loadCompareXLSXStyles(pkg *comparePackage, rels map[string]compareRelationship) (*compareXLSXStyles, error) {
	styles := &compareXLSXStyles{formats: map[string]string{}}
	for _, rel := range rels {
		if !rel.external && strings.HasSuffix(rel.kind, "/styles") {
			root, err := pkg.xml(rel.target)
			if err != nil {
				return nil, err
			}
			styles.fonts = root.child("fonts").all("font")
			styles.fills = root.child("fills").all("fill")
			styles.borders = root.child("borders").all("border")
			styles.xfs = root.child("cellXfs").all("xf")
			styles.parents = root.child("cellStyleXfs").all("xf")
			styles.dxfs = root.child("dxfs").all("dxf")
			for _, format := range root.child("numFmts").all("numFmt") {
				styles.formats[format.attr("numFmtId")] = format.attr("formatCode")
			}
		}
	}
	return styles, nil
}
func (s *compareXLSXStyles) expand(xf *compareNode, parent bool) (*compareNode, error) {
	if xf == nil {
		return nil, nil
	}
	root := &compareNode{name: xml.Name{Space: xf.name.Space, Local: "cell-format"}}
	attrs := append([]xml.Attr(nil), xf.attrs...)
	sort.Slice(attrs, func(i, j int) bool { return attrs[i].Name.Local < attrs[j].Name.Local })
	for _, attr := range attrs {
		var options []*compareNode
		switch attr.Name.Local {
		case "fontId":
			options = s.fonts
		case "fillId":
			options = s.fills
		case "borderId":
			options = s.borders
		case "numFmtId":
			value := s.formats[attr.Value]
			if value == "" {
				value = "builtin:" + attr.Value
			}
			attr.Value = value
			root.attrs = append(root.attrs, attr)
			continue
		case "xfId":
			if parent {
				continue
			}
			index, e := strconv.Atoi(attr.Value)
			if e != nil || index < 0 || index >= len(s.parents) {
				return nil, fmt.Errorf("invalid parent cell style")
			}
			base, e := s.expand(s.parents[index], true)
			if e != nil {
				return nil, e
			}
			root.children = append(root.children, base)
			root.parts = append(root.parts, base)
			continue
		default:
			root.attrs = append(root.attrs, attr)
			continue
		}
		index, e := strconv.Atoi(attr.Value)
		if e != nil || index < 0 || index >= len(options) {
			return nil, fmt.Errorf("invalid referenced cell appearance")
		}
		component := projectCompareNode(options[index], nil)
		root.children = append(root.children, component)
		root.parts = append(root.parts, component)
	}
	for _, child := range xf.children {
		root.children = append(root.children, child)
		root.parts = append(root.parts, child)
	}
	return root, nil
}
func (s *compareXLSXStyles) style(value string) (*compareNode, error) {
	if value == "" {
		value = "0"
	}
	index, e := strconv.Atoi(value)
	if e != nil || index < 0 {
		return nil, fmt.Errorf("invalid cell appearance index")
	}
	if len(s.xfs) == 0 && index == 0 {
		return nil, nil
	}
	if index >= len(s.xfs) {
		return nil, fmt.Errorf("cell appearance missing")
	}
	return s.expand(s.xfs[index], false)
}
func readCompareXLSXDetails(pkg *comparePackage, doc *structuredDocument) error {
	book, err := pkg.xml("xl/workbook.xml")
	if err != nil {
		return err
	}
	rels, err := pkg.relationships("xl/workbook.xml")
	if err != nil {
		return err
	}
	styles, err := loadCompareXLSXStyles(pkg, rels)
	if err != nil {
		return err
	}
	for _, rel := range rels {
		if !rel.external && strings.HasSuffix(rel.kind, "/theme") {
			root, e := pkg.xml(rel.target)
			if e != nil {
				return e
			}
			canon, e := newDetailCanonical(pkg, rel.target, root)
			if e != nil {
				return e
			}
			if e = addCompareDetail(doc, "workbook-theme", "", "theme", "", 0, canon, root); e != nil {
				return e
			}
		}
	}
	for index, sheet := range book.child("sheets").all("sheet") {
		name := sheet.attr("name")
		rel := rels[sheet.relationshipID()]
		body, e := pkg.xml(rel.target)
		if e != nil {
			return e
		}
		canon, e := newDetailCanonical(pkg, rel.target, body)
		if e != nil {
			return e
		}
		canon.resolve = func(node *compareNode, attr xml.Attr) (string, bool, error) {
			if attr.Name.Local == "dxfId" {
				n, e := strconv.Atoi(attr.Value)
				if e != nil || n < 0 || n >= len(styles.dxfs) {
					return "", true, fmt.Errorf("conditional appearance missing")
				}
				plain := *canon
				plain.resolve = nil
				_, key, e := plain.describe(styles.dxfs[n])
				return "conditional-style:" + key, true, e
			}
			return "", false, nil
		}
		defaultStyle, e := styles.style("0")
		if e != nil {
			return e
		}
		_, defaultKey, e := canon.describe(defaultStyle)
		if e != nil {
			return e
		}
		if index == 0 {
			if e = addCompareDetail(doc, "workbook-default-style", "", "cell-style", "Default", 0, canon, defaultStyle); e != nil {
				return e
			}
		}
		layout := detailContainer("sheet-layout")
		for _, tag := range []string{"sheetFormatPr", "sheetViews", "mergeCells", "pageMargins", "pageSetup", "printOptions", "conditionalFormatting", "autoFilter", "sheetPr"} {
			for _, node := range body.all(tag) {
				layout.children = append(layout.children, node)
				layout.parts = append(layout.parts, node)
			}
		}
		// Active selection/cursors do not change saved appearance.
		layout = projectCompareNode(layout, map[string]bool{"selection": true, "pane": false})
		if e = addCompareDetail(doc, "sheet-layout\x00"+name, name, "sheet-layout", "", index+1, canon, layout); e != nil {
			return e
		}
		rowNumber := 0
		for _, row := range body.child("sheetData").all("row") {
			rowNumber++
			if value := row.attr("r"); value != "" {
				rowNumber, e = strconv.Atoi(value)
				if e != nil {
					return e
				}
			}
			if row.attr("r") == "" {
				cells := row.all("c")
				if len(cells) > 0 && cells[0].attr("r") != "" {
					rowNumber, _, e = parseCompareAddress(cells[0].attr("r"))
					if e != nil {
						return e
					}
				}
			}
			rowProps := &compareNode{name: xml.Name{Local: "row-properties"}}
			for _, attr := range row.attrs {
				if attr.Name.Local == "r" || attr.Name.Local == "spans" {
					continue
				}
				if attr.Name.Local == "s" {
					style, e := styles.style(attr.Value)
					if e != nil {
						return e
					}
					if style != nil {
						rowProps.parts = append(rowProps.parts, style)
						rowProps.children = append(rowProps.children, style)
					}
					continue
				}
				rowProps.attrs = append(rowProps.attrs, attr)
			}
			if e = addCompareDetail(doc, fmt.Sprintf("row-style\x00%s\x00%d", name, rowNumber), name, "row-style", strconv.Itoa(rowNumber), index+1, canon, rowProps); e != nil {
				return e
			}
			column := 0
			for _, cell := range row.all("c") {
				column++
				r, c := rowNumber, column
				if address := cell.attr("r"); address != "" {
					r, c, e = parseCompareAddress(address)
					if e != nil {
						return e
					}
					column = c
				}
				if cell.attr("s") == "" || cell.attr("s") == "0" {
					continue
				}
				style, e := styles.style(cell.attr("s"))
				if e != nil {
					return e
				}
				_, styleKey, e := canon.describe(style)
				if e != nil {
					return e
				}
				if styleKey == defaultKey {
					continue
				}
				address := compareCellAddress(r, c)
				if e = addCompareDetail(doc, "cell-style\x00"+name+"\x00"+address, name, "cell-style", address, index+1, canon, style); e != nil {
					return e
				}
			}
		}
		for _, column := range body.child("cols").all("col") {
			props := projectCompareNode(column, nil)
			attrs := props.attrs[:0]
			for _, attr := range props.attrs {
				if attr.Name.Local == "style" {
					style, e := styles.style(attr.Value)
					if e != nil {
						return e
					}
					if style != nil {
						props.parts = append(props.parts, style)
						props.children = append(props.children, style)
					}
				} else {
					attrs = append(attrs, attr)
				}
			}
			props.attrs = attrs
			address := column.attr("min") + ":" + column.attr("max")
			if e = addCompareDetail(doc, "column-style\x00"+name+"\x00"+address, name, "column-style", address, index+1, canon, props); e != nil {
				return e
			}
		}
		drawingRels, e := pkg.relationships(rel.target)
		if e != nil {
			return e
		}
		chartIndex := 0
		for _, drawing := range body.all("drawing") {
			link, ok := drawingRels[drawing.relationshipID()]
			if !ok || link.external {
				return fmt.Errorf("worksheet drawing missing")
			}
			shapes, e := pkg.xml(link.target)
			if e != nil {
				return e
			}
			shapeCanon, e := newDetailCanonical(pkg, link.target, shapes)
			if e != nil {
				return e
			}
			if e = addCompareDetail(doc, "sheet-drawing\x00"+name+"\x00"+strconv.Itoa(chartIndex), name, "sheet-layout", "Objects", index+1, shapeCanon, projectCompareNode(shapes, map[string]bool{"t": true})); e != nil {
				return e
			}
			chartIndex, e = addCompareCharts(pkg, doc, link.target, shapes, name, "sheet-chart", index+1, chartIndex)
			if e != nil {
				return e
			}
		}
	}
	return nil
}
func addCompareCharts(pkg *comparePackage, doc *structuredDocument, part string, root *compareNode, group, section string, index, start int, identity ...string) (int, error) {
	rels, err := pkg.relationships(part)
	if err != nil {
		return start, err
	}
	for _, chart := range root.descendants("chart") {
		id := chart.relationshipID()
		if id == "" {
			continue
		}
		rel, ok := rels[id]
		if !ok || rel.external || !strings.HasSuffix(rel.kind, "/chart") {
			return start, fmt.Errorf("chart relationship missing or external")
		}
		tree, e := pkg.xml(rel.target)
		if e != nil {
			return start, e
		}
		canonical, e := newDetailCanonical(pkg, rel.target, tree)
		if e != nil {
			return start, e
		}
		report := detailContainer("chart-properties", tree)
		dependencies, e := pkg.relationships(rel.target)
		if e != nil {
			return start, e
		}
		roles := []string{}
		byRole := map[string]compareRelationship{}
		for _, dependency := range dependencies {
			role := path.Base(dependency.kind)
			if role != "chartStyle" && role != "chartColorStyle" {
				continue
			}
			if dependency.external {
				return start, fmt.Errorf("external chart style unsupported")
			}
			if _, ok := byRole[role]; ok {
				return start, fmt.Errorf("duplicate chart style dependency")
			}
			roles = append(roles, role)
			byRole[role] = dependency
		}
		sort.Strings(roles)
		for _, role := range roles {
			dependency := byRole[role]
			style, e := pkg.xml(dependency.target)
			if e != nil {
				return start, e
			}
			cc, e := newDetailCanonical(pkg, dependency.target, style)
			if e != nil {
				return start, e
			}
			display, _, e := cc.describe(style)
			if e != nil {
				return start, e
			}
			node := detailContainer(role)
			node.parts = []any{display}
			report.children = append(report.children, node)
			report.parts = append(report.parts, node)
		}
		start++
		// Formula caches in chart XML are displayed chart data, not calculation cache
		// on worksheet cells, and must be included in this comparison.
		keyGroup := group
		if len(identity) > 0 {
			keyGroup = identity[0]
		}
		if e = addCompareDetail(doc, section+"\x00"+keyGroup+"\x00"+strconv.Itoa(start), group, section, strconv.Itoa(start), index, canonical, report); e != nil {
			return start, e
		}
	}
	return start, nil
}
