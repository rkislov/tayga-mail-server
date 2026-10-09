package noteutil

import (
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"net/url"
	"strconv"
	"strings"
)

// Parse each block independently: lists must never absorb surrounding paragraphs.
func DocumentFromHTML(source string) Node {
	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	children, err := html.ParseFragment(strings.NewReader(source), root)
	if err != nil {
		return DocumentFromText(stripTags(source))
	}
	doc := Node{Type: "doc", Content: noteBlocks(children)}
	if len(doc.Content) == 0 {
		return EmptyDoc()
	}
	return doc
}
func noteAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func noteHasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}
func noteChildren(n *html.Node) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = append(out, c)
	}
	return out
}
func noteIgnored(n *html.Node) bool {
	switch n.Data {
	case "script", "style", "iframe", "object", "svg", "template":
		return true
	}
	return false
}
func noteBlocks(nodes []*html.Node) []Node {
	var out []Node
	var inline []Node
	flush := func() {
		if len(inline) > 0 {
			out = append(out, Node{Type: "paragraph", Content: inline})
			inline = nil
		}
	}
	for _, n := range nodes {
		if n.Type == html.TextNode {
			if strings.TrimSpace(n.Data) != "" {
				inline = append(inline, Node{Type: "text", Text: n.Data})
			}
			continue
		}
		if n.Type != html.ElementNode || noteIgnored(n) {
			continue
		}
		switch n.Data {
		case "ul", "ol":
			flush()
			out = append(out, noteList(n))
		case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote":
			flush()
			if noteAttr(n, "data-type") == "drawing" {
				out = append(out, Node{Type: "drawing", Attrs: map[string]any{"attachment_id": noteAttr(n, "data-attachment-id"), "preview_id": noteAttr(n, "data-preview-id")}})
				continue
			}
			if noteContainsBlocks(n) {
				out = append(out, noteBlocks(noteChildren(n))...)
			} else {
				block := Node{Type: "paragraph", Content: noteInline(noteChildren(n), nil)}
				if strings.HasPrefix(n.Data, "h") && len(n.Data) == 2 {
					level, _ := strconv.Atoi(n.Data[1:])
					block.Type = "heading"
					block.Attrs = map[string]any{"level": level}
				}
				out = append(out, block)
			}
		default:
			inline = append(inline, noteInline([]*html.Node{n}, nil)...)
		}
	}
	flush()
	return out
}
func noteContainsBlocks(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Data {
		case "p", "div", "ul", "ol":
			return true
		}
	}
	return false
}
func noteList(n *html.Node) Node {
	kind := "bulletList"
	if n.Data == "ol" {
		kind = "orderedList"
	}
	task := noteAttr(n, "data-type") == "checklist" || noteAttr(n, "data-type") == "taskList"
	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.Data == "li" && (noteAttr(li, "data-type") == "taskItem" || noteCheckbox(li) != nil) {
			task = true
		}
	}
	if task {
		kind = "taskList"
	}
	list := Node{Type: kind}
	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}
		item := Node{Type: "listItem", Content: noteBlocks(noteChildren(li))}
		if len(item.Content) == 0 {
			item.Content = []Node{{Type: "paragraph"}}
		}
		if task {
			checked := noteAttr(li, "data-checked") == "true"
			if cb := noteCheckbox(li); cb != nil {
				checked = checked || noteHasAttr(cb, "checked")
			}
			item.Type = "taskItem"
			item.Attrs = map[string]any{"checked": checked}
		}
		// Legacy rendered checklists used Unicode boxes without data attributes.
		if len(item.Content) > 0 && len(item.Content[0].Content) > 0 {
			text := &item.Content[0].Content[0]
			if strings.HasPrefix(text.Text, "☑ ") || strings.HasPrefix(text.Text, "☐ ") {
				list.Type = "taskList"
				item.Type = "taskItem"
				item.Attrs = map[string]any{"checked": strings.HasPrefix(text.Text, "☑ ")}
				text.Text = strings.TrimPrefix(strings.TrimPrefix(text.Text, "☑ "), "☐ ")
			}
		}
		list.Content = append(list.Content, item)
	}
	return list
}
func noteCheckbox(n *html.Node) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Data == "input" && noteAttr(c, "type") == "checkbox" {
			return c
		}
		if c.Data != "ul" && c.Data != "ol" {
			if found := noteCheckbox(c); found != nil {
				return found
			}
		}
	}
	return nil
}
func noteInline(nodes []*html.Node, marks []Mark) []Node {
	var out []Node
	for _, n := range nodes {
		if n.Type == html.TextNode {
			out = append(out, Node{Type: "text", Text: n.Data, Marks: append([]Mark(nil), marks...)})
			continue
		}
		if n.Type != html.ElementNode || noteIgnored(n) || n.Data == "input" {
			continue
		}
		next := append([]Mark(nil), marks...)
		switch n.Data {
		case "br":
			out = append(out, Node{Type: "hardBreak"})
			continue
		case "b", "strong":
			next = append(next, Mark{Type: "bold"})
		case "i", "em":
			next = append(next, Mark{Type: "italic"})
		case "u":
			next = append(next, Mark{Type: "underline"})
		case "a":
			href := noteAttr(n, "href")
			if noteSafeURL(href) {
				next = append(next, Mark{Type: "link", Attrs: map[string]any{"href": href}})
			}
		}
		out = append(out, noteInline(noteChildren(n), next)...)
	}
	return out
}
func noteSafeURL(href string) bool {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return false
	}
	return u.Scheme == "" || u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "mailto"
}
