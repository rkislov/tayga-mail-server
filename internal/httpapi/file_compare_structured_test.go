package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func comparisonPackageFixture(t *testing.T, parts map[string]string) string {
	t.Helper()
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	for name, content := range parts {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.String()
}
func xlsxComparisonFixture(t *testing.T, cells, shared string) string {
	return comparisonPackageFixture(t, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="План" sheetId="1" r:id="sheet"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="sheet" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/one.xml"/><Relationship Id="strings" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/></Relationships>`,
		"xl/sharedStrings.xml":       `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` + shared + `</sst>`,
		"xl/worksheets/one.xml":      `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` + cells + `</sheetData></worksheet>`,
	})
}
func structuredResult(t *testing.T, f *fileFixture, path, old, current string) []structuredChange {
	t.Helper()
	rev := makeCompareHistory(t, f, path, old, current)
	response := f.call(0, "POST", "/api/v1/files/library/compare", compareRequest(path, rev.ID, "current", "structured"), 200)
	var result struct {
		Mode, Scope string
		Changes     []structuredChange
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Mode != "structured" {
		t.Fatal(response.Body.String())
	}
	return result.Changes
}
func TestSpreadsheetComparisonXLSXCellsAndCacheNormalization(t *testing.T) {
	f := newFileFixture(t)
	old := xlsxComparisonFixture(t, `<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1"><v>1.0</v></c><c r="C1"><f>SUM(B1,2)</f><v>3</v></c><c r="D1" t="inlineStr"><is><t>old</t></is></c><c r="F1" t="d"><v>2026-11-05</v></c></row>`, `<si><t>same</t></si><si><t>unused</t></si>`)
	current := xlsxComparisonFixture(t, `<row r="1"><c r="A1" t="s"><v>1</v></c><c r="B1"><v>1</v></c><c r="C1"><f>SUM(B1,2)</f><v>99</v></c><c r="D1" t="inlineStr"><is><r><t>new</t></r></is></c><c r="E1" t="b"><v>1</v></c></row>`, `<si><t>unused</t></si><si><r><t>same</t></r></si>`)
	changes := structuredResult(t, f, "plan.xlsx", old, current)
	if len(changes) != 3 {
		t.Fatal(changes)
	}
	byAddress := map[string]structuredChange{}
	for _, change := range changes {
		byAddress[change.Address] = change
		if change.Group != "План" {
			t.Fatal(change)
		}
	}
	if byAddress["D1"].Kind != "change" || byAddress["D1"].Before != "old" || byAddress["D1"].After != "new" || byAddress["E1"].Kind != "insert" || byAddress["F1"].Kind != "delete" {
		t.Fatal(changes)
	}
	if f.call(0, "GET", "/dav/files/plan.xlsx", "", 200).Body.String() != current {
		t.Fatal("comparison changed spreadsheet")
	}
}
func TestSpreadsheetComparisonSharedFormulaAndChangedFormula(t *testing.T) {
	f := newFileFixture(t)
	old := xlsxComparisonFixture(t, `<row r="2"><c r="C2"><f t="shared" si="7" ref="C2:C3">A2+$B$1</f><v>1</v></c></row><row r="3"><c r="C3"><f t="shared" si="7"/><v>2</v></c></row>`, "")
	expanded := xlsxComparisonFixture(t, `<row r="2"><c r="C2"><f>A2+$B$1</f></c></row><row r="3"><c r="C3"><f>A3+$B$1</f></c></row>`, "")
	if changes := structuredResult(t, f, "formulas.xlsx", old, expanded); len(changes) != 0 {
		t.Fatal(changes)
	}
	changed := xlsxComparisonFixture(t, `<row r="2"><c r="C2"><f>A2+$B$1</f></c></row><row r="3"><c r="C3"><f>A3+$B$2</f></c></row>`, "")
	changes := structuredResult(t, f, "changed.xlsx", expanded, changed)
	if len(changes) != 1 || changes[0].Address != "C3" || changes[0].After != "=A3+$B$2" {
		t.Fatal(changes)
	}
}
func TestSpreadsheetComparisonODSRepeatedCells(t *testing.T) {
	f := newFileFixture(t)
	prefix := `<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0"><office:body><office:spreadsheet><table:table table:name="Budget">`
	end := `<table:table-row table:number-rows-repeated="1048575"><table:table-cell table:number-columns-repeated="16384"/></table:table-row></table:table></office:spreadsheet></office:body></office:document-content>`
	old := prefix + `<table:table-row><table:table-cell office:value-type="string" table:number-columns-repeated="2"><text:p>same</text:p></table:table-cell><table:table-cell office:value-type="float" office:value="1.0"/><table:table-cell office:value-type="float" office:value="3" table:formula="of:=SUM([.C1];2)"/></table:table-row>` + end
	current := prefix + `<table:table-row><table:table-cell office:value-type="string"><text:p>same</text:p></table:table-cell><table:table-cell office:value-type="string"><text:p>new</text:p></table:table-cell><table:table-cell office:value-type="float" office:value="1"/><table:table-cell office:value-type="float" office:value="99" table:formula="of:=SUM([.C1];2)"/></table:table-row>` + end
	old = strings.Replace(old, `</table:table-row>`, `<table:table-cell office:value-type="boolean" office:boolean-value="1"/></table:table-row>`, 1)
	current = strings.Replace(current, `</table:table-row>`, `<table:table-cell office:value-type="boolean" office:boolean-value="true"/></table:table-row>`, 1)
	changes := structuredResult(t, f, "budget.ods", comparisonPackageFixture(t, map[string]string{"content.xml": old}), comparisonPackageFixture(t, map[string]string{"content.xml": current}))
	if len(changes) != 1 || changes[0].Address != "B1" || changes[0].After != "new" {
		t.Fatal(changes)
	}
}
func presentationFixture(t *testing.T, order []string, text, image string) string {
	ids, rels := "", ""
	for _, id := range order {
		ids += `<p:sldId id="` + id + `" r:id="rel` + id + `"/>`
		rels += `<Relationship Id="rel` + id + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide` + id + `.xml"/>`
	}
	parts := map[string]string{"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst>` + ids + `</p:sldIdLst></p:presentation>`, "ppt/_rels/presentation.xml.rels": `<Relationships>` + rels + `</Relationships>`}
	for _, id := range order {
		body := text
		if id != "256" {
			body = "other slide"
		}
		parts["ppt/slides/slide"+id+".xml"] = `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>` + body + `</a:t></a:r></a:p></p:txBody></p:sp>`
		if id == "256" {
			parts["ppt/slides/slide"+id+".xml"] += `<p:pic><a:blip r:embed="picture"/></p:pic>`
			parts["ppt/slides/_rels/slide"+id+".xml.rels"] = `<Relationships><Relationship Id="picture" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/picture.png"/></Relationships>`
			parts["ppt/media/picture.png"] = image
		}
		parts["ppt/slides/slide"+id+".xml"] += `</p:spTree></p:cSld></p:sld>`
	}
	return comparisonPackageFixture(t, parts)
}
func TestPresentationComparisonPPTXOrderTextAndImage(t *testing.T) {
	f := newFileFixture(t)
	old := presentationFixture(t, []string{"256", "257"}, "same text", "old image")
	current := presentationFixture(t, []string{"257", "256"}, "same text", "new image")
	changes := structuredResult(t, f, "slides.pptx", old, current)
	sections := map[string]int{}
	for _, change := range changes {
		sections[change.Section]++
	}
	if len(changes) != 3 || sections["slide"] != 2 || sections["slide-images"] != 1 {
		t.Fatal(changes)
	}
	changed := presentationFixture(t, []string{"256", "257"}, "new text", "old image")
	changes = structuredResult(t, f, "text-slides.pptx", old, changed)
	if len(changes) != 1 || changes[0].Section != "slide-text" || changes[0].Before != "same text" || changes[0].After != "new text" {
		t.Fatal(changes)
	}
}
func TestPresentationComparisonODPAndImageNameNormalization(t *testing.T) {
	f := newFileFixture(t)
	fixture := func(text, file, bytes string) string {
		return comparisonPackageFixture(t, map[string]string{"content.xml": `<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:xlink="http://www.w3.org/1999/xlink"><office:body><office:presentation><draw:page draw:name="Title"><draw:frame><draw:text-box><text:p>` + text + `</text:p></draw:text-box></draw:frame><draw:frame><draw:image xlink:href="` + file + `"/></draw:frame></draw:page></office:presentation></office:body></office:document-content>`, file: bytes})
	}
	old := fixture("same", "Pictures/a.png", "image")
	if changes := structuredResult(t, f, "renamed-image.odp", old, fixture("same", "Pictures/b.png", "image")); len(changes) != 0 {
		t.Fatal(changes)
	}
	changes := structuredResult(t, f, "updated-image.odp", old, fixture("same", "Pictures/b.png", "changed"))
	if len(changes) != 1 || changes[0].Section != "slide-images" {
		t.Fatal(changes)
	}
}
func TestRebaseCompareFormula(t *testing.T) {
	cases := map[string]string{`A1+$B$2+C$3+$D4`: `B2+$B$2+D$3+$D5`, `SUM(A:A,$B:$C,1:2,$3:$4)`: `SUM(B:B,$B:$C,2:3,$3:$4)`, `IF(A1="A1",'A1 sheet'!B2,Table1[A1])`: `IF(B2="A1",'A1 sheet'!C3,Table1[A1])`, `LOG10(A1)`: `LOG10(B2)`}
	for input, want := range cases {
		if got := rebaseCompareFormula(input, 1, 1); got != want {
			t.Fatal(input, got, want)
		}
	}
}
func TestStructuredComparisonRejectsBrokenAndUnsupported(t *testing.T) {
	f := newFileFixture(t)
	old := xlsxComparisonFixture(t, `<row r="1"><c r="A1"><v>1</v></c></row>`, "")
	rev := makeCompareHistory(t, f, "bad.xlsx", old, "not an archive")
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("bad.xlsx", rev.ID, "current", "structured"), 422)
	array := xlsxComparisonFixture(t, `<row r="1"><c r="A1"><f t="array" ref="A1:A2">SUM(B1:B2)</f></c></row>`, "")
	f.call(0, "PUT", "/api/v1/files/content?path=bad.xlsx", array, 200)
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("bad.xlsx", rev.ID, "current", "structured"), 422)
	f.call(1, "POST", "/api/v1/files/library/compare?owner_id="+f.users[0], compareRequest("bad.xlsx", rev.ID, "current", "structured"), 403)
	f.call(1, "POST", "/api/v1/files/library/compare", compareRequest("bad.xlsx", rev.ID, "current", "structured"), 404)
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("bad.xlsx", rev.ID, "current", "office"), 415)
}

func TestStructuredComparisonBuiltinOfficePackages(t *testing.T) {
	f := newFileFixture(t)
	for _, item := range []struct{ id, ext string }{{"spreadsheet", ".xlsx"}, {"presentation", ".pptx"}} {
		data, err := officeTemplates.ReadFile("file_templates/" + item.id + item.ext)
		if err != nil {
			t.Fatal(err)
		}
		if changes := structuredResult(t, f, item.id+item.ext, string(data), string(data)); len(changes) != 0 {
			t.Fatal(changes)
		}
	}
}
func TestStructuredComparisonSheetMetadataAndMissingSlide(t *testing.T) {
	f := newFileFixture(t)
	cells := `<row r="1"><c r="A1"><v>1</v></c></row>`
	// Change workbook metadata without changing any cell or its ZIP representation.
	base := func(hidden string) string {
		return comparisonPackageFixture(t, map[string]string{
			"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Budget" r:id="one" state="` + hidden + `"/></sheets></workbook>`,
			"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="one" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/a.xml"/></Relationships>`,
			"xl/worksheets/a.xml":        `<worksheet><sheetData>` + cells + `</sheetData></worksheet>`,
		})
	}
	changes := structuredResult(t, f, "hidden.xlsx", base("visible"), base("hidden"))
	if len(changes) != 1 || changes[0].Section != "sheet" {
		t.Fatal(changes)
	}
	old := presentationFixture(t, []string{"256", "257"}, "same", "image")
	current := presentationFixture(t, []string{"256"}, "same", "image")
	changes = structuredResult(t, f, "deleted-slide.pptx", old, current)
	if len(changes) != 2 {
		t.Fatal(changes)
	}
	for _, change := range changes {
		if change.Kind != "delete" {
			t.Fatal(changes)
		}
	}
}
func TestStructuredComparisonBoundedExpansionAndNumbers(t *testing.T) {
	f := newFileFixture(t)
	safe := xlsxComparisonFixture(t, `<row r="1"><c r="A1"><v>1</v></c></row>`, "")
	rev := makeCompareHistory(t, f, "limits.xlsx", safe, xlsxComparisonFixture(t, `<row r="1"><c r="A1"><v>1E9999999999</v></c></row>`, ""))
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("limits.xlsx", rev.ID, "current", "structured"), 422)
	// The aggregate record limit rejects large populated sheets, with no truncation.
	var rows strings.Builder
	for i := 1; i <= maxStructuredEntries; i++ {
		rows.WriteString(`<row r="` + strconv.Itoa(i) + `"><c r="A` + strconv.Itoa(i) + `"><v>1</v></c></row>`)
	}
	f.call(0, "PUT", "/api/v1/files/content?path=limits.xlsx", xlsxComparisonFixture(t, rows.String(), ""), 200)
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("limits.xlsx", rev.ID, "current", "structured"), 422)
}
