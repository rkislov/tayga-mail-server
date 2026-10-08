package noteutil

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
)

// Node is a TipTap/ProseMirror-compatible document node (subset).
type Node struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
}

// Mark is an inline mark (bold, link, …).
type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// EmptyDoc returns a minimal empty document.
func EmptyDoc() Node {
	return Node{Type: "doc", Content: []Node{{Type: "paragraph"}}}
}

// ParseDocumentJSON parses document JSON; empty input yields EmptyDoc.
func ParseDocumentJSON(raw string) Node {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return EmptyDoc()
	}
	var n Node
	if err := json.Unmarshal([]byte(raw), &n); err != nil || n.Type == "" {
		return DocumentFromText(raw)
	}
	if n.Type != "doc" {
		return Node{Type: "doc", Content: []Node{n}}
	}
	return n
}

// MarshalDocumentJSON encodes a document.
func MarshalDocumentJSON(n Node) string {
	if n.Type == "" {
		n = EmptyDoc()
	}
	b, err := json.Marshal(n)
	if err != nil {
		return `{"type":"doc","content":[{"type":"paragraph"}]}`
	}
	return string(b)
}

// DocumentFromText wraps plain text as paragraphs.
func DocumentFromText(text string) Node {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	parts := strings.Split(text, "\n")
	doc := Node{Type: "doc"}
	for _, p := range parts {
		para := Node{Type: "paragraph"}
		if p != "" {
			para.Content = []Node{{Type: "text", Text: p}}
		}
		doc.Content = append(doc.Content, para)
	}
	if len(doc.Content) == 0 {
		doc.Content = []Node{{Type: "paragraph"}}
	}
	return doc
}

// DocumentFromHTML builds a coarse document from sticky-note HTML.
func DocumentFromHTML(h string) Node {
	h = strings.TrimSpace(h)
	if h == "" {
		return EmptyDoc()
	}
	// Strip scripts/styles
	reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	h = reScript.ReplaceAllString(h, "")
	h = reStyle.ReplaceAllString(h, "")

	doc := Node{Type: "doc"}
	// Split on block boundaries
	reBlock := regexp.MustCompile(`(?i)</?(p|div|br|li|ul|ol|h[1-6])[^>]*>`)
	chunks := reBlock.Split(h, -1)
	inList := strings.Contains(strings.ToLower(h), "<ul") || strings.Contains(strings.ToLower(h), "<ol")
	inTask := strings.Contains(strings.ToLower(h), "checkbox") || strings.Contains(h, "☐") || strings.Contains(h, "☑")

	if inTask {
		taskList := Node{Type: "taskList"}
		for _, c := range chunks {
			t := strings.TrimSpace(stripTags(c))
			if t == "" {
				continue
			}
			checked := strings.HasPrefix(t, "[x]") || strings.HasPrefix(t, "[X]") || strings.HasPrefix(t, "☑")
			t = strings.TrimPrefix(t, "[x]")
			t = strings.TrimPrefix(t, "[X]")
			t = strings.TrimPrefix(t, "[ ]")
			t = strings.TrimPrefix(t, "☑")
			t = strings.TrimPrefix(t, "☐")
			t = strings.TrimSpace(t)
			item := Node{
				Type:  "taskItem",
				Attrs: map[string]any{"checked": checked},
				Content: []Node{{
					Type: "paragraph",
					Content: []Node{{Type: "text", Text: t}},
				}},
			}
			taskList.Content = append(taskList.Content, item)
		}
		if len(taskList.Content) > 0 {
			doc.Content = append(doc.Content, taskList)
			return doc
		}
	}

	if inList {
		list := Node{Type: "bulletList"}
		for _, c := range chunks {
			t := strings.TrimSpace(stripTags(c))
			if t == "" {
				continue
			}
			list.Content = append(list.Content, Node{
				Type: "listItem",
				Content: []Node{{
					Type: "paragraph",
					Content: []Node{{Type: "text", Text: t}},
				}},
			})
		}
		if len(list.Content) > 0 {
			doc.Content = append(doc.Content, list)
			return doc
		}
	}

	for _, c := range chunks {
		t := strings.TrimSpace(stripTags(c))
		if t == "" {
			continue
		}
		doc.Content = append(doc.Content, Node{
			Type: "paragraph",
			Content: []Node{{Type: "text", Text: t}},
		})
	}
	if len(doc.Content) == 0 {
		return DocumentFromText(stripTags(h))
	}
	return doc
}

func stripTags(s string) string {
	re := regexp.MustCompile(`(?s)<[^>]*>`)
	s = re.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(s)
}

// ToHTML renders document JSON/HTML for FlowSync clients.
func ToHTML(doc Node) string {
	if doc.Type == "" {
		doc = EmptyDoc()
	}
	var b strings.Builder
	renderHTML(&b, doc)
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "<p></p>"
	}
	return out
}

func renderHTML(b *strings.Builder, n Node) {
	switch n.Type {
	case "doc":
		for _, c := range n.Content {
			renderHTML(b, c)
		}
	case "paragraph":
		b.WriteString("<p>")
		renderInline(b, n.Content)
		b.WriteString("</p>")
	case "heading":
		level := 2
		if n.Attrs != nil {
			if v, ok := n.Attrs["level"].(float64); ok {
				level = int(v)
			}
		}
		if level < 1 || level > 6 {
			level = 2
		}
		b.WriteString("<h" + itoa(level) + ">")
		renderInline(b, n.Content)
		b.WriteString("</h" + itoa(level) + ">")
	case "bulletList":
		b.WriteString("<ul>")
		for _, c := range n.Content {
			renderHTML(b, c)
		}
		b.WriteString("</ul>")
	case "orderedList":
		b.WriteString("<ol>")
		for _, c := range n.Content {
			renderHTML(b, c)
		}
		b.WriteString("</ol>")
	case "listItem":
		b.WriteString("<li>")
		for _, c := range n.Content {
			renderHTML(b, c)
		}
		b.WriteString("</li>")
	case "taskList":
		b.WriteString("<ul>")
		for _, c := range n.Content {
			renderHTML(b, c)
		}
		b.WriteString("</ul>")
	case "taskItem":
		checked := false
		if n.Attrs != nil {
			if v, ok := n.Attrs["checked"].(bool); ok {
				checked = v
			}
		}
		mark := "☐ "
		if checked {
			mark = "☑ "
		}
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(mark))
		for _, c := range n.Content {
			if c.Type == "paragraph" {
				renderInline(b, c.Content)
			} else {
				renderHTML(b, c)
			}
		}
		b.WriteString("</li>")
	case "drawing":
		b.WriteString("<p>[drawing]</p>")
	case "text":
		renderInline(b, []Node{n})
	default:
		for _, c := range n.Content {
			renderHTML(b, c)
		}
	}
}

func renderInline(b *strings.Builder, nodes []Node) {
	for _, n := range nodes {
		if n.Type != "text" {
			renderHTML(b, n)
			continue
		}
		t := html.EscapeString(n.Text)
		for _, m := range n.Marks {
			switch m.Type {
			case "bold", "strong":
				t = "<strong>" + t + "</strong>"
			case "italic", "em":
				t = "<em>" + t + "</em>"
			case "link":
				href := ""
				if m.Attrs != nil {
					if v, ok := m.Attrs["href"].(string); ok {
						href = v
					}
				}
				if href != "" {
					t = `<a href="` + html.EscapeString(href) + `">` + t + `</a>`
				}
			}
		}
		b.WriteString(t)
	}
}

// ToText extracts plain text.
func ToText(doc Node) string {
	var b strings.Builder
	collectText(&b, doc)
	return strings.TrimSpace(b.String())
}

func collectText(b *strings.Builder, n Node) {
	if n.Type == "text" {
		b.WriteString(n.Text)
		return
	}
	if n.Type == "paragraph" || n.Type == "heading" || n.Type == "listItem" || n.Type == "taskItem" {
		for _, c := range n.Content {
			collectText(b, c)
		}
		b.WriteByte('\n')
		return
	}
	for _, c := range n.Content {
		collectText(b, c)
	}
}

// DeriveBodies returns html and text from document JSON.
func DeriveBodies(documentJSON string) (htmlOut, textOut string) {
	doc := ParseDocumentJSON(documentJSON)
	return ToHTML(doc), ToText(doc)
}

// DocumentFromHTMLOrText picks HTML parser when markup present.
func DocumentFromHTMLOrText(body string) Node {
	body = strings.TrimSpace(body)
	if body == "" {
		return EmptyDoc()
	}
	if strings.Contains(body, "<") && strings.Contains(body, ">") {
		return DocumentFromHTML(body)
	}
	return DocumentFromText(body)
}

func itoa(n int) string {
	if n < 0 {
		return "0"
	}
	var digits []byte
	if n == 0 {
		return "0"
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
