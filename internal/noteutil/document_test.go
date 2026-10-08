package noteutil_test

import (
	"strings"
	"testing"

	"github.com/tayga/tms/internal/noteutil"
)

func TestDocumentRoundTrip(t *testing.T) {
	doc := noteutil.DocumentFromText("line1\nline2")
	raw := noteutil.MarshalDocumentJSON(doc)
	if !strings.Contains(raw, `"type":"doc"`) {
		t.Fatalf("raw=%s", raw)
	}
	html, text := noteutil.DeriveBodies(raw)
	if !strings.Contains(html, "line1") || !strings.Contains(text, "line2") {
		t.Fatalf("html=%q text=%q", html, text)
	}
	fromHTML := noteutil.DocumentFromHTMLOrText("<p>hi <b>there</b></p>")
	if fromHTML.Type != "doc" {
		t.Fatalf("type=%s", fromHTML.Type)
	}
}
