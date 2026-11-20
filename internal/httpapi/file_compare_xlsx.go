package httpapi

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func richCompareText(node *compareNode) string {
	var out strings.Builder
	if node == nil {
		return ""
	}
	for _, part := range node.children {
		switch part.name.Local {
		case "t":
			out.WriteString(part.text())
		case "r":
			out.WriteString(richCompareText(part))
		}
	}
	return out.String()
}
func readCompareXLSX(pkg *comparePackage, doc *structuredDocument) error {
	book, err := pkg.xml("xl/workbook.xml")
	if err != nil {
		return err
	}
	if book.name.Local != "workbook" {
		return fmt.Errorf("invalid workbook")
	}
	rels, err := pkg.relationships("xl/workbook.xml")
	if err != nil {
		return err
	}
	stringsPart, stylesPart := "", ""
	for _, rel := range rels {
		if rel.external {
			continue
		}
		if strings.HasSuffix(rel.kind, "/sharedStrings") {
			stringsPart = rel.target
		}
		if strings.HasSuffix(rel.kind, "/styles") {
			stylesPart = rel.target
		}
	}
	shared := []string{}
	if stringsPart != "" {
		table, e := pkg.xml(stringsPart)
		if e != nil {
			return e
		}
		for _, entry := range table.all("si") {
			shared = append(shared, richCompareText(entry))
			if len(shared) > 100000 {
				return fmt.Errorf("too many shared strings")
			}
		}
	}
	formats := []string{}
	if stylesPart != "" {
		styles, e := pkg.xml(stylesPart)
		if e != nil {
			return e
		}
		custom := map[string]string{}
		for _, format := range styles.child("numFmts").all("numFmt") {
			custom[format.attr("numFmtId")] = format.attr("formatCode")
		}
		for _, xf := range styles.child("cellXfs").all("xf") {
			id := xf.attr("numFmtId")
			format := custom[id]
			if format == "" && id != "" && id != "0" {
				format = "builtin:" + id
			}
			formats = append(formats, format)
		}
	}
	dateSystem := "1900"
	if value := book.child("workbookPr").attr("date1904"); value == "1" || value == "true" {
		dateSystem = "1904"
	}
	if err = doc.add("workbook-date", structuredEntry{"", "workbook", "", dateSystem, dateSystem, "0000", 0}); err != nil {
		return err
	}
	sheets := book.child("sheets").all("sheet")
	if len(sheets) == 0 {
		return fmt.Errorf("workbook has no sheets")
	}
	for index, sheet := range sheets {
		name, state := sheet.attr("name"), sheet.attr("state")
		if state == "" {
			state = "visible"
		}
		if err = compareSheet(doc, name, state, index+1); err != nil {
			return err
		}
		rel, ok := rels[sheet.relationshipID()]
		if !ok || rel.external || !strings.HasSuffix(rel.kind, "/worksheet") {
			return fmt.Errorf("unsupported sheet relationship")
		}
		body, e := pkg.xml(rel.target)
		if e != nil {
			return e
		}
		if body.name.Local != "worksheet" {
			return fmt.Errorf("invalid worksheet")
		}
		type sharedFormula struct {
			formula     string
			row, column int
		}
		masters := map[string]sharedFormula{}
		for _, row := range body.child("sheetData").all("row") {
			for _, cell := range row.all("c") {
				f := cell.child("f")
				if f == nil || f.attr("t") != "shared" || f.text() == "" {
					continue
				}
				r, c, e := parseCompareAddress(cell.attr("r"))
				if e != nil {
					return e
				}
				si := f.attr("si")
				if _, exists := masters[si]; exists {
					return fmt.Errorf("duplicate shared formula master")
				}
				masters[si] = sharedFormula{f.text(), r, c}
			}
		}
		rowNumber := 0
		for _, row := range body.child("sheetData").all("row") {
			rowNumber++
			if row.attr("r") == "" {
				cells := row.all("c")
				if len(cells) > 0 && cells[0].attr("r") != "" {
					rowNumber, _, e = parseCompareAddress(cells[0].attr("r"))
					if e != nil {
						return e
					}
				}
			}
			if row.attr("r") != "" {
				rowNumber, e = strconv.Atoi(row.attr("r"))
				if e != nil || rowNumber < 1 || rowNumber > 1048576 {
					return fmt.Errorf("invalid worksheet row")
				}
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
					if r != rowNumber {
						return fmt.Errorf("cell does not belong to its row")
					}
					column = c
				}
				value, kind := cell.child("v").text(), "number"
				cellType := cell.attr("t")
				if cell.child("f") != nil {
					cellType = "n"
				}
				switch cellType {
				case "s":
					i, e := strconv.Atoi(value)
					if e != nil || i < 0 || i >= len(shared) {
						return fmt.Errorf("invalid shared string reference")
					}
					value, kind = shared[i], "string"
				case "inlineStr":
					value, kind = richCompareText(cell.child("is")), "string"
				case "str":
					kind = "string"
				case "b":
					kind = "boolean"
					switch value {
					case "0":
						value = "false"
					case "1":
						value = "true"
					default:
						return fmt.Errorf("invalid boolean cell")
					}
				case "e":
					kind = "error"
				case "d":
					kind = "date"
				case "", "n":
				default:
					return fmt.Errorf("unsupported cell type")
				}
				formula := ""
				if f := cell.child("f"); f != nil {
					formula = f.text()
					switch f.attr("t") {
					case "shared":
						master, ok := masters[f.attr("si")]
						if !ok {
							return fmt.Errorf("shared formula master missing")
						}
						formula = rebaseCompareFormula(master.formula, r-master.row, c-master.column)
					case "", "normal":
					case "array":
						return fmt.Errorf("array formulas are unsupported in this comparison version")
					default:
						return fmt.Errorf("unsupported formula type")
					}
					if formula == "" {
						return fmt.Errorf("empty formula")
					}
				}
				format := ""
				if style := cell.attr("s"); style != "" {
					n, e := strconv.Atoi(style)
					if e != nil || n < 0 || (n >= len(formats) && !(n == 0 && len(formats) == 0)) {
						return fmt.Errorf("invalid cell style")
					}
					if n < len(formats) {
						format = formats[n]
					}
				}
				if e = compareCell(doc, name, index+1, r, c, kind, value, formula, format); e != nil {
					return e
				}
			}
		}
	}
	return nil
}

// Shared formula groups are normalized to per-cell A1 expressions. Quoted
// literals, sheet names and structured references are kept verbatim; no formula
// is evaluated and no external workbook is opened.
var compareRowRange = regexp.MustCompile(`^(\$?)([1-9][0-9]*):(\$?)([1-9][0-9]*)`)
var compareColumnRange = regexp.MustCompile(`^(\$?)([A-Za-z]{1,3}):(\$?)([A-Za-z]{1,3})`)

func rebaseCompareFormula(formula string, rowDelta, columnDelta int) string {
	var out strings.Builder
	for i := 0; i < len(formula); {
		ch := formula[i]
		if ch == '"' || ch == '\'' {
			quote := ch
			start := i
			i++
			for i < len(formula) {
				if formula[i] == quote {
					i++
					if i < len(formula) && formula[i] == quote {
						i++
						continue
					}
					break
				}
				i++
			}
			out.WriteString(formula[start:i])
			continue
		}
		if ch == '[' {
			start := i
			depth := 1
			i++
			for i < len(formula) && depth > 0 {
				if formula[i] == '[' {
					depth++
				}
				if formula[i] == ']' {
					depth--
				}
				i++
			}
			out.WriteString(formula[start:i])
			continue
		}
		if i > 0 && formulaIdentifier(formula[i-1]) {
			out.WriteByte(ch)
			i++
			continue
		}
		if match := compareRowRange.FindStringSubmatch(formula[i:]); match != nil && (i+len(match[0]) == len(formula) || !formulaIdentifier(formula[i+len(match[0])])) {
			parts := []string{}
			for _, offset := range []int{1, 3} {
				number, e := strconv.Atoi(match[offset+1])
				if e != nil || number > 1048576 {
					parts = append(parts, "#REF!")
					continue
				}
				if match[offset] != "$" {
					number += rowDelta
				}
				if number < 1 || number > 1048576 {
					parts = append(parts, "#REF!")
				} else {
					parts = append(parts, match[offset]+strconv.Itoa(number))
				}
			}
			out.WriteString(strings.Join(parts, ":"))
			i += len(match[0])
			continue
		}
		if match := compareColumnRange.FindStringSubmatch(formula[i:]); match != nil && (i+len(match[0]) == len(formula) || !formulaIdentifier(formula[i+len(match[0])])) {
			parts := []string{}
			for _, offset := range []int{1, 3} {
				_, column, e := parseCompareAddress(strings.ToUpper(match[offset+1]) + "1")
				if e != nil {
					parts = append(parts, "#REF!")
					continue
				}
				if match[offset] != "$" {
					column += columnDelta
				}
				if column < 1 || column > 16384 {
					parts = append(parts, "#REF!")
				} else {
					parts = append(parts, match[offset]+strings.TrimSuffix(compareCellAddress(1, column), "1"))
				}
			}
			out.WriteString(strings.Join(parts, ":"))
			i += len(match[0])
			continue
		}
		j := i
		absColumn := false
		if j < len(formula) && formula[j] == '$' {
			absColumn = true
			j++
		}
		colStart := j
		for j < len(formula) && isFormulaLetter(formula[j]) {
			j++
		}
		if j == colStart || j-colStart > 3 {
			out.WriteByte(ch)
			i++
			continue
		}
		letters := strings.ToUpper(formula[colStart:j])
		absRow := false
		if j < len(formula) && formula[j] == '$' {
			absRow = true
			j++
		}
		rowStart := j
		for j < len(formula) && formula[j] >= '0' && formula[j] <= '9' {
			j++
		}
		if j == rowStart || (j < len(formula) && (formulaIdentifier(formula[j]) || formula[j] == '(' || formula[j] == '!')) {
			out.WriteByte(ch)
			i++
			continue
		}
		r, c, e := parseCompareAddress(letters + formula[rowStart:j])
		if e != nil {
			out.WriteByte(ch)
			i++
			continue
		}
		if !absColumn {
			c += columnDelta
		}
		if !absRow {
			r += rowDelta
		}
		if r < 1 || r > 1048576 || c < 1 || c > 16384 {
			out.WriteString("#REF!")
		} else {
			address := compareCellAddress(r, c)
			n := 0
			for n < len(address) && isFormulaLetter(address[n]) {
				n++
			}
			if absColumn {
				out.WriteByte('$')
			}
			out.WriteString(address[:n])
			if absRow {
				out.WriteByte('$')
			}
			out.WriteString(address[n:])
		}
		i = j
	}
	return out.String()
}
func isFormulaLetter(ch byte) bool { return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' }
func formulaIdentifier(ch byte) bool {
	return isFormulaLetter(ch) || ch >= '0' && ch <= '9' || ch == '_' || ch == '.'
}
