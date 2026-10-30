package noteutil

import (
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"strings"
)

// ValidateSyncReplacement refuses a lossy conversion of structured content by
// clients that do not understand it. An explicitly empty body can clear a note;
// the previous document is retained by the storage revision history.
func ValidateSyncReplacement(previous, next Node) error {
	if ToText(next) == "" && len(next.Content) <= 1 && (len(next.Content) == 0 || next.Content[0].Type == "paragraph" && len(next.Content[0].Content) == 0) {
		return nil
	}
	tasks := func(doc Node) int {
		count := 0
		var walk func(Node)
		walk = func(n Node) {
			if n.Type == "taskItem" {
				count++
			}
			for _, c := range n.Content {
				walk(c)
			}
		}
		walk(doc)
		return count
	}
	if tasks(previous) > tasks(next) {
		return fmt.Errorf("client removed checklist structure; edit this note in Tayga")
	}
	drawings := func(doc Node) map[string]int {
		out := map[string]int{}
		var walk func(Node)
		walk = func(n Node) {
			if n.Type == "drawing" {
				id, _ := n.Attrs["attachment_id"].(string)
				preview, _ := n.Attrs["preview_id"].(string)
				out[id+"\x00"+preview]++
			}
			for _, c := range n.Content {
				walk(c)
			}
		}
		walk(doc)
		return out
	}
	before, after := drawings(previous), drawings(next)
	if len(before) != len(after) {
		return fmt.Errorf("client changed drawing references; edit this note in Tayga")
	}
	for key, count := range before {
		if after[key] != count {
			return fmt.Errorf("client removed drawing structure; edit this note in Tayga")
		}
	}
	return nil
}

// DocumentFromSyncHTML rejects content the document model cannot retain. In
// particular an image-only body must not be mistaken for an explicit clear.
func DocumentFromSyncHTML(source string) (Node, error) {
	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(source), root)
	if err != nil {
		return Node{}, err
	}
	var inspect func(*html.Node) error
	inspect = func(n *html.Node) error {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "img", "svg", "table", "video", "audio", "canvas", "object", "embed", "iframe":
				return fmt.Errorf("unsupported note content %s; edit this note in Tayga", n.Data)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if err := inspect(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, n := range nodes {
		if err := inspect(n); err != nil {
			return Node{}, err
		}
	}
	return DocumentFromHTML(source), nil
}
