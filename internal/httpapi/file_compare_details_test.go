package httpapi

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func detailedComparison(t *testing.T, f *fileFixture, name, old, current string) []structuredChange {
	t.Helper()
	rev := makeCompareHistory(t, f, name, old, current)
	response := f.call(0, "POST", "/api/v1/files/library/compare", compareRequest(name, rev.ID, "current", "details"), 200)
	var data struct {
		Details bool
		Changes []structuredChange
	}
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil || !data.Details {
		t.Fatal(response.Body.String(), err)
	}
	return data.Changes
}
func appearanceXLSXFixture(t *testing.T, size string, reindexed bool) string {
	font := func(size string) string {
		return `<font><name val="Arial"/><sz val="` + size + `"/><color rgb="FF112233"/></font>`
	}
	fonts := font("11") + font(size)
	defaultID, selectedID, cellStyle := "0", "1", "1"
	if reindexed {
		fonts = font(size) + font("11")
		defaultID, selectedID, cellStyle = "1", "0", "2"
	}
	xf := func(fontID string) string {
		return `<xf fontId="` + fontID + `" fillId="0" borderId="0" numFmtId="0" xfId="0"/>`
	}
	xfs := xf(defaultID) + xf(selectedID)
	if reindexed {
		xfs = xf(defaultID) + xf(defaultID) + xf(selectedID)
	}
	return comparisonPackageFixture(t, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Plan" r:id="sheet"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="sheet" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/a.xml"/><Relationship Id="style" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/worksheets/a.xml":        `<worksheet><sheetData><row r="1"><c r="A1" s="` + cellStyle + `" t="inlineStr"><is><t>same value</t></is></c></row></sheetData></worksheet>`,
		"xl/styles.xml":              `<styleSheet><fonts>` + fonts + `</fonts><fills><fill><patternFill patternType="none"/></fill></fills><borders><border/></borders><cellStyleXfs><xf fontId="` + defaultID + `" fillId="0" borderId="0" numFmtId="0"/></cellStyleXfs><cellXfs>` + xfs + `</cellXfs></styleSheet>`,
	})
}
func TestDetailedComparisonXLSXAppearanceAndStyleRenumber(t *testing.T) {
	f := newFileFixture(t)
	old := appearanceXLSXFixture(t, "14", false)
	if changes := detailedComparison(t, f, "reindexed.xlsx", old, appearanceXLSXFixture(t, "14", true)); len(changes) != 0 {
		t.Fatal(changes)
	}
	changes := detailedComparison(t, f, "font.xlsx", old, appearanceXLSXFixture(t, "18", true))
	if len(changes) != 1 || changes[0].Section != "cell-style" || changes[0].Address != "A1" || !strings.Contains(changes[0].After, "18") {
		t.Fatal(changes)
	}
}
func appearancePPTXFixture(t *testing.T, font, duration, transition, shapeID, timeID string) string {
	return comparisonPackageFixture(t, map[string]string{
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="one"/></p:sldIdLst><p:sldSz cx="9144000" cy="6858000"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships><Relationship Id="one" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/a.xml"/></Relationships>`,
		"ppt/slides/a.xml":                `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="` + shapeID + `" name="Title"/></p:nvSpPr><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="1000000" cy="2000000"/></a:xfrm></p:spPr><p:txBody><a:p><a:r><a:rPr sz="` + font + `"/><a:t>same text</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld><p:transition spd="fast"><p:` + transition + `/></p:transition><p:timing><p:tnLst><p:par><p:cTn id="` + timeID + `" dur="` + duration + `"><p:childTnLst><p:anim><p:cBhvr><p:tgtEl><p:spTgt spid="` + shapeID + `"/></p:tgtEl></p:cBhvr></p:anim></p:childTnLst></p:cTn></p:par></p:tnLst></p:timing></p:sld>`,
	})
}
func TestDetailedComparisonPPTXFontTimingTransitionAndIDs(t *testing.T) {
	f := newFileFixture(t)
	old := appearancePPTXFixture(t, "2400", "200", "fade", "7", "10")
	if changes := detailedComparison(t, f, "ids.pptx", old, appearancePPTXFixture(t, "2400", "200", "fade", "88", "600")); len(changes) != 0 {
		t.Fatal(changes)
	}
	for _, test := range []struct{ name, font, duration, transition, section string }{{"font", "2800", "200", "fade", "slide-style"}, {"timing", "2400", "400", "fade", "slide-animation"}, {"transition", "2400", "200", "push", "slide-transition"}} {
		changes := detailedComparison(t, f, test.name+".pptx", old, appearancePPTXFixture(t, test.font, test.duration, test.transition, "7", "10"))
		if len(changes) != 1 || changes[0].Section != test.section {
			t.Fatal(test.name, changes)
		}
	}
}
func chartXLSXFixture(t *testing.T, value, axis1, axis2, relationship string) string {
	return comparisonPackageFixture(t, map[string]string{
		"xl/workbook.xml":                `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Plan" r:id="sheet"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels":     `<Relationships><Relationship Id="sheet" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/a.xml"/></Relationships>`,
		"xl/worksheets/a.xml":            `<worksheet xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheetData/><drawing r:id="drawing"/></worksheet>`,
		"xl/worksheets/_rels/a.xml.rels": `<Relationships><Relationship Id="drawing" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/a.xml"/></Relationships>`,
		"xl/drawings/a.xml":              `<wsDr xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><graphicFrame><c:chart r:id="` + relationship + `"/></graphicFrame></wsDr>`,
		"xl/drawings/_rels/a.xml.rels":   `<Relationships><Relationship Id="` + relationship + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/a.xml"/></Relationships>`,
		"xl/charts/a.xml":                `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart><c:plotArea><c:barChart><c:ser><c:val><c:numRef><c:f>Plan!A1</c:f><c:numCache><c:pt idx="0"><c:v>` + value + `</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser><c:axId val="` + axis1 + `"/><c:axId val="` + axis2 + `"/></c:barChart><c:catAx><c:axId val="` + axis1 + `"/><c:crossAx val="` + axis2 + `"/></c:catAx><c:valAx><c:axId val="` + axis2 + `"/><c:crossAx val="` + axis1 + `"/></c:valAx></c:plotArea></c:chart></c:chartSpace>`,
	})
}
func TestDetailedComparisonChartDataAndServiceReferences(t *testing.T) {
	f := newFileFixture(t)
	old := chartXLSXFixture(t, "3", "100", "200", "chart1")
	if changes := detailedComparison(t, f, "chart-ids.xlsx", old, chartXLSXFixture(t, "3", "400", "500", "renamed")); len(changes) != 0 {
		t.Fatal(changes)
	}
	changes := detailedComparison(t, f, "chart-data.xlsx", old, chartXLSXFixture(t, "9", "100", "200", "chart1"))
	found := false
	for _, change := range changes {
		if change.Section == "sheet-chart" && strings.Contains(change.After, "= 9") {
			found = true
		}
	}
	if !found {
		t.Fatal(changes)
	}
}
func appearanceODPFixture(t *testing.T, style, font, duration, shapeID string) string {
	return comparisonPackageFixture(t, map[string]string{"content.xml": `<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0" xmlns:anim="urn:oasis:names:tc:opendocument:xmlns:animation:1.0" xmlns:smil="urn:oasis:names:tc:opendocument:xmlns:smil-compatible:1.0"><office:automatic-styles><style:style style:name="` + style + `" style:family="graphic"><style:text-properties fo:font-size="` + font + `"/></style:style></office:automatic-styles><office:body><office:presentation><draw:page draw:name="Title"><draw:frame xml:id="` + shapeID + `" draw:style-name="` + style + `"><draw:text-box><text:p>same text</text:p></draw:text-box></draw:frame><anim:par smil:dur="` + duration + `" smil:targetElement="` + shapeID + `"/></draw:page></office:presentation></office:body></office:document-content>`})
}
func TestDetailedComparisonODPStyleReferencesAndAnimation(t *testing.T) {
	f := newFileFixture(t)
	old := appearanceODPFixture(t, "G1", "12pt", "1s", "shape1")
	if changes := detailedComparison(t, f, "odf-ids.odp", old, appearanceODPFixture(t, "G9", "12pt", "1s", "shape7")); len(changes) != 0 {
		t.Fatal(changes)
	}
	changes := detailedComparison(t, f, "odf-font.odp", old, appearanceODPFixture(t, "G1", "16pt", "1s", "shape1"))
	found := false
	for _, change := range changes {
		if change.Section == "theme" && strings.Contains(change.After, "16pt") {
			found = true
		}
	}
	if !found {
		t.Fatal(changes)
	}
	changes = detailedComparison(t, f, "odf-time.odp", old, appearanceODPFixture(t, "G1", "12pt", "2s", "shape1"))
	if len(changes) != 1 || changes[0].Section != "slide-animation" {
		t.Fatal(changes)
	}
}
func TestDetailedComparisonBuiltinPackages(t *testing.T) {
	f := newFileFixture(t)
	for _, item := range []struct{ id, ext string }{{"spreadsheet", ".xlsx"}, {"presentation", ".pptx"}} {
		data, err := officeTemplates.ReadFile("file_templates/" + item.id + item.ext)
		if err != nil {
			t.Fatal(err)
		}
		if changes := detailedComparison(t, f, "builtin"+item.ext, string(data), string(data)); len(changes) != 0 {
			t.Fatal(fmt.Sprint(changes))
		}
	}
}

func TestDetailedComparisonODSCellFormatting(t *testing.T) {
	f := newFileFixture(t)
	fixture := func(size string) string {
		return comparisonPackageFixture(t, map[string]string{"content.xml": `<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0"><office:automatic-styles><style:style style:name="C1" style:family="table-cell"><style:text-properties fo:font-size="` + size + `"/></style:style></office:automatic-styles><office:body><office:spreadsheet><table:table table:name="Plan"><table:table-row><table:table-cell table:style-name="C1" office:value-type="float" office:value="1"/></table:table-row></table:table></office:spreadsheet></office:body></office:document-content>`})
	}
	changes := detailedComparison(t, f, "ods-font.ods", fixture("12pt"), fixture("18pt"))
	found := false
	for _, change := range changes {
		if change.Section == "theme" && strings.Contains(change.After, "18pt") {
			found = true
		}
	}
	if !found {
		t.Fatal(changes)
	}
}
func TestDetailedComparisonODFEmbeddedChart(t *testing.T) {
	f := newFileFixture(t)
	fixture := func(value string) string {
		return comparisonPackageFixture(t, map[string]string{
			"content.xml":         `<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:xlink="http://www.w3.org/1999/xlink"><office:body><office:presentation><draw:page draw:name="Title"><draw:frame><draw:object xlink:href="./Object1"/></draw:frame></draw:page></office:presentation></office:body></office:document-content>`,
			"Object1/content.xml": `<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:chart="urn:oasis:names:tc:opendocument:xmlns:chart:1.0" xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0"><office:body><office:chart><chart:chart chart:class="chart:bar"><chart:plot-area/><table:table table:name="local"><table:table-row><table:table-cell office:value-type="float" office:value="` + value + `"/></table:table-row></table:table></chart:chart></office:chart></office:body></office:document-content>`,
		})
	}
	changes := detailedComparison(t, f, "odf-chart.odp", fixture("3"), fixture("9"))
	found := false
	for _, change := range changes {
		if change.Section == "slide-chart" && strings.Contains(change.After, "= 9") {
			found = true
		}
	}
	if !found {
		t.Fatal(changes)
	}
}
func TestDetailedComparisonInvalidStyleCycleAndAccess(t *testing.T) {
	f := newFileFixture(t)
	old := appearanceODPFixture(t, "G1", "12pt", "1s", "shape1")
	var cyclic string
	// Build a valid ZIP whose visible style refers to itself.
	var content string
	stream, err := zip.NewReader(strings.NewReader(old), int64(len(old)))
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range stream.File {
		if part.Name == "content.xml" {
			reader, e := part.Open()
			if e != nil {
				t.Fatal(e)
			}
			data, e := io.ReadAll(reader)
			reader.Close()
			if e != nil {
				t.Fatal(e)
			}
			content = strings.Replace(string(data), `style:family="graphic"`, `style:family="graphic" style:parent-style-name="G1"`, 1)
		}
	}
	cyclic = comparisonPackageFixture(t, map[string]string{"content.xml": content})
	rev := makeCompareHistory(t, f, "cyclic.odp", old, cyclic)
	request := compareRequest("cyclic.odp", rev.ID, "current", "details")
	f.call(0, "POST", "/api/v1/files/library/compare", request, 422)
	f.call(1, "POST", "/api/v1/files/library/compare?owner_id="+f.users[0], request, 403)
	if f.call(0, "GET", "/dav/files/cyclic.odp", "", 200).Body.String() != cyclic {
		t.Fatal("comparison changed file")
	}
}
