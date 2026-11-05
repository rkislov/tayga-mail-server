package httpapi

import (
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
)

func odfCompareText(n *compareNode) (string, error) {
	if n == nil {
		return "", nil
	}
	if n.name.Local == "annotation" || n.name.Local == "tracked-changes" || n.name.Local == "note" || n.name.Local == "notes" {
		return "", nil
	}
	switch n.name.Local {
	case "s":
		count := 1
		if v := n.attr("c"); v != "" {
			var e error
			count, e = strconv.Atoi(v)
			if e != nil || count < 1 || count > maxCompareTextBytes {
				return "", fmt.Errorf("invalid ODF whitespace count")
			}
		}
		return strings.Repeat(" ", count), nil
	case "tab":
		return "\t", nil
	case "line-break":
		return "\n", nil
	}
	var out strings.Builder
	for _, part := range n.parts {
		switch value := part.(type) {
		case string:
			out.WriteString(value)
		case *compareNode:
			text, err := odfCompareText(value)
			if err != nil {
				return "", err
			}
			out.WriteString(text)
		}
		if out.Len() > maxCompareTextBytes {
			return "", fmt.Errorf("ODF text exceeds comparison limit")
		}
	}
	return out.String(), nil
}
func odfParagraphs(node *compareNode) (string, error) {
	var out []string
	var visit func(*compareNode) error
	visit = func(n *compareNode) error {
		if n == nil || n.name.Local == "annotation" || n.name.Local == "tracked-changes" || n.name.Local == "note" || n.name.Local == "notes" {
			return nil
		}
		if n.name.Local == "p" || n.name.Local == "h" {
			text, err := odfCompareText(n)
			if err != nil {
				return err
			}
			out = append(out, text)
			return nil
		}
		for _, child := range n.children {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(node); err != nil {
		return "", err
	}
	text := strings.Join(out, "\n")
	if len(text) > maxCompareTextBytes {
		return "", fmt.Errorf("ODF text exceeds comparison limit")
	}
	return text, nil
}
func odfRepeat(node *compareNode, name string, limit int) (int, error) {
	value := node.attr(name)
	if value == "" {
		return 1, nil
	}
	n, e := strconv.Atoi(value)
	if e != nil || n < 1 || n > limit {
		return 0, fmt.Errorf("ODF repetition exceeds comparison limit")
	}
	return n, nil
}
func readCompareODS(pkg *comparePackage, doc *structuredDocument) error {
	root, err := pkg.xml("content.xml")
	if err != nil {
		return err
	}
	book := root.child("body").child("spreadsheet")
	if book == nil {
		return fmt.Errorf("spreadsheet body missing")
	}
	if len(book.all("table")) == 0 {
		return fmt.Errorf("spreadsheet has no sheets")
	}
	// Date origin affects every serial date even when cells themselves are unchanged.
	origin := "1899-12-30"
	for _, date := range book.descendants("null-date") {
		if date.attr("date-value") != "" {
			origin = date.attr("date-value")
		}
	}
	if err = doc.add("workbook-date", structuredEntry{"", "workbook", "", origin, origin, "0000", 0}); err != nil {
		return err
	}
	for index, sheet := range book.all("table") {
		name := sheet.attr("name")
		state := "visible"
		if sheet.attr("display") == "false" {
			state = "hidden"
		}
		if err = compareSheet(doc, name, state, index+1); err != nil {
			return err
		}
		// Row/header/group containers do not change cell coordinates.
		rows := []*compareNode{}
		var collect func(*compareNode)
		collect = func(node *compareNode) {
			for _, child := range node.children {
				switch child.name.Local {
				case "table-row":
					rows = append(rows, child)
				case "table-row-group", "table-rows", "table-header-rows":
					collect(child)
				}
			}
		}
		collect(sheet)
		rowIndex := 1
		for _, row := range rows {
			repeat, e := odfRepeat(row, "number-rows-repeated", 1048576)
			if e != nil {
				return e
			}
			if rowIndex+repeat-1 > 1048576 {
				return fmt.Errorf("too many ODF rows")
			}
			type cellValue struct {
				column, count        int
				kind, value, formula string
			}
			values := []cellValue{}
			column := 1
			for _, cell := range row.children {
				if cell.name.Local != "table-cell" && cell.name.Local != "covered-table-cell" {
					continue
				}
				count, e := odfRepeat(cell, "number-columns-repeated", 16384)
				if e != nil {
					return e
				}
				if column+count-1 > 16384 {
					return fmt.Errorf("too many ODF columns")
				}
				if cell.name.Local == "covered-table-cell" {
					column += count
					continue
				}
				kind := cell.attr("value-type")
				formula := cell.attr("formula")
				value := ""
				switch kind {
				case "float":
					kind = "number"
					value = cell.attr("value")
				case "percentage", "currency":
					value = cell.attr("value")
					if kind == "currency" && value != "" {
						value += " " + cell.attr("currency")
					}
				case "boolean":
					value = cell.attr("boolean-value")
					switch value {
					case "1", "true":
						value = "true"
					case "0", "false":
						value = "false"
					default:
						return fmt.Errorf("invalid ODF boolean cell")
					}
				case "date":
					value = cell.attr("date-value")
				case "time":
					value = cell.attr("time-value")
				case "string", "":
					kind = "string"
					value = cell.attr("string-value")
					if value == "" {
						value, e = odfParagraphs(cell)
						if e != nil {
							return e
						}
					}
				default:
					return fmt.Errorf("unsupported ODF cell type")
				}
				if formula != "" {
					if strings.HasPrefix(formula, "of:=") {
						formula = strings.TrimPrefix(formula, "of:=")
					} else {
						formula = strings.TrimPrefix(formula, "=")
					}
					if count > 1 || repeat > 1 {
						return fmt.Errorf("repeated ODF formula cells are unsupported")
					}
				}
				if value != "" || formula != "" {
					values = append(values, cellValue{column, count, kind, value, formula})
				}
				column += count
			}
			populated := 0
			for _, value := range values {
				populated += value.count
			}
			if populated > 0 && repeat > (maxStructuredEntries-len(doc.entries))/populated {
				return fmt.Errorf("populated ODF repetitions exceed comparison limit")
			}
			// Empty repeated rows are skipped without expansion (LibreOffice commonly
			// stores the remaining million blank rows as a single repeated row).
			if populated > 0 {
				for r := 0; r < repeat; r++ {
					for _, cell := range values {
						for c := 0; c < cell.count; c++ {
							if e = compareCell(doc, name, index+1, rowIndex+r, cell.column+c, cell.kind, cell.value, cell.formula, ""); e != nil {
								return e
							}
						}
					}
				}
			}
			rowIndex += repeat
		}
	}
	return nil
}
func readCompareODP(pkg *comparePackage, doc *structuredDocument) error {
	root, err := pkg.xml("content.xml")
	if err != nil {
		return err
	}
	body := root.child("body").child("presentation")
	if body == nil {
		return fmt.Errorf("presentation body missing")
	}
	for index, slide := range body.all("page") {
		id := slide.attr("id")
		if id == "" {
			id = slide.attr("name")
		}
		text, e := odfParagraphs(slide)
		if e != nil {
			return e
		}
		images := []string{}
		for _, image := range compareODFImages(slide) {
			// Inline base64 image data is not silently ignored.
			if image.child("binary-data") != nil {
				return fmt.Errorf("inline ODF images are unsupported")
			}
			target := image.attr("href")
			if target == "" {
				return fmt.Errorf("ODF image target missing")
			}
			parsed, e := url.Parse(target)
			if e != nil {
				return fmt.Errorf("invalid ODF image target")
			}
			if parsed.IsAbs() || parsed.Host != "" {
				images = append(images, "external: "+target)
				continue
			}
			target = path.Clean(strings.TrimPrefix(parsed.Path, "./"))
			if target == ".." || strings.HasPrefix(target, "../") || strings.HasPrefix(target, "/") {
				return fmt.Errorf("ODF image escapes package")
			}
			hash, e := compareImage(pkg, target)
			if e != nil {
				return e
			}
			images = append(images, hash)
		}
		if e = compareSlide(doc, id, slide.attr("name"), text, images, index+1, slide.attr("visibility") == "hidden"); e != nil {
			return e
		}
	}
	return nil
}

func compareODFImages(node *compareNode) []*compareNode {
	var out []*compareNode
	var visit func(*compareNode)
	visit = func(current *compareNode) {
		if current.name.Local == "notes" || current.name.Local == "annotation" {
			return
		}
		if current.name.Local == "image" {
			out = append(out, current)
			return
		}
		for _, child := range current.children {
			visit(child)
		}
	}
	visit(node)
	return out
}
