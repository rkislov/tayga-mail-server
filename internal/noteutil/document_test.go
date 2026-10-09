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

func TestHTMLKeepsMixedBlocksAndTaskState(t *testing.T) {
	source := `<h2>Title</h2><p>Before <strong>bold</strong></p><ul data-type="checklist"><li data-type="taskItem" data-checked="true"><input type="checkbox" checked><div data-task-text><p>Done</p></div></li><li><input type="checkbox"><div data-task-text><br></div></li></ul><p>Between</p><ol><li>One</li><li>Two<ul><li>Nested</li></ul></li></ol><p>After<br>next</p><div data-type="drawing" data-attachment-id="draw-1" contenteditable="false">✎</div>`
	doc := noteutil.DocumentFromHTML(source)
	if len(doc.Content) != 7 {
		t.Fatalf("blocks: %#v", doc.Content)
	}
	if doc.Content[2].Type != "taskList" || doc.Content[2].Content[0].Attrs["checked"] != true || len(doc.Content[2].Content) != 2 {
		t.Fatalf("task state: %#v", doc.Content[2])
	}
	if doc.Content[4].Type != "orderedList" || doc.Content[4].Content[1].Content[1].Type != "bulletList" {
		t.Fatal("list hierarchy lost")
	}
	rendered, _ := noteutil.DeriveBodies(noteutil.MarshalDocumentJSON(doc))
	again := noteutil.DocumentFromHTML(rendered)
	if again.Content[2].Content[0].Attrs["checked"] != true || again.Content[6].Attrs["attachment_id"] != "draw-1" {
		t.Fatalf("round trip lost state: %s", rendered)
	}
	if !strings.Contains(rendered, "<strong>bold</strong>") || !strings.Contains(rendered, "<ol>") || !strings.Contains(rendered, "<br>") {
		t.Fatalf("formatting lost: %s", rendered)
	}
}

func TestHTMLDropsUnsafeContentAndLinks(t *testing.T) {
	doc := noteutil.DocumentFromHTML(`<p onclick="alert(1)">Safe<script>alert(2)</script><a href="javascript:alert(3)">bad</a><a href="https://example.com">good</a></p><iframe>hidden</iframe>`)
	rendered := noteutil.ToHTML(doc)
	if strings.Contains(rendered, "javascript:") || strings.Contains(rendered, "alert(") || strings.Contains(rendered, "hidden") {
		t.Fatalf("unsafe: %s", rendered)
	}
	if !strings.Contains(rendered, `href="https://example.com"`) {
		t.Fatal("safe link lost")
	}
}
