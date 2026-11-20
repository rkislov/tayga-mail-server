package httpapi

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
)

const maxStructuredEntries = 10000

type comparePackage struct {
	files        map[string]*zip.File
	cache        map[string][]byte
	used         int64
	detailHashes map[string]string
}

func openComparePackage(f *os.File) (*comparePackage, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxCompareFileBytes {
		return nil, fmt.Errorf("comparison file exceeds 32 MiB")
	}
	archive, err := zip.NewReader(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("invalid office package")
	}
	if len(archive.File) > 10000 {
		return nil, fmt.Errorf("too many office package entries")
	}
	pkg := &comparePackage{files: map[string]*zip.File{}, cache: map[string][]byte{}}
	for _, file := range archive.File {
		name := file.Name
		if strings.HasSuffix(name, "/") {
			continue
		}
		if path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || strings.Contains(name, "\\") || pkg.files[name] != nil {
			return nil, fmt.Errorf("invalid or duplicate office package entry")
		}
		pkg.files[name] = file
	}
	return pkg, nil
}
func (p *comparePackage) read(name string) ([]byte, error) {
	if data, ok := p.cache[name]; ok {
		return data, nil
	}
	file := p.files[name]
	if file == nil {
		return nil, fmt.Errorf("office part missing: %s", name)
	}
	if len(p.cache) >= 512 || file.UncompressedSize64 > 16<<20 || p.used+int64(file.UncompressedSize64) > 32<<20 {
		return nil, fmt.Errorf("office package exceeds comparison expansion limit")
	}
	stream, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, (16<<20)+1))
	if err != nil {
		return nil, fmt.Errorf("invalid office part: %s", name)
	}
	if len(data) > 16<<20 || p.used+int64(len(data)) > 32<<20 {
		return nil, fmt.Errorf("office package exceeds comparison expansion limit")
	}
	p.used += int64(len(data))
	p.cache[name] = data
	return data, nil
}
func (p *comparePackage) xml(name string) (*compareNode, error) {
	data, err := p.read(name)
	if err != nil {
		return nil, err
	}
	if len(data) > 8<<20 {
		return nil, fmt.Errorf("office XML exceeds 8 MiB")
	}
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	var root *compareNode
	var stack []*compareNode
	nodes := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid office XML: %s", name)
		}
		switch node := token.(type) {
		case xml.StartElement:
			nodes++
			if len(stack) >= 128 || nodes > 100000 {
				return nil, fmt.Errorf("office XML exceeds comparison complexity limit")
			}
			current := &compareNode{name: node.Name, attrs: node.Attr}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("multiple XML roots")
				}
				root = current
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, current)
				parent.parts = append(parent.parts, current)
			}
			stack = append(stack, current)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("invalid XML nesting")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.parts = append(parent.parts, string(node))
			} else if strings.TrimSpace(string(node)) != "" {
				return nil, fmt.Errorf("text outside XML root")
			}
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, fmt.Errorf("incomplete office XML")
	}
	return root, nil
}

type compareNode struct {
	name     xml.Name
	attrs    []xml.Attr
	children []*compareNode
	parts    []any
}

func (n *compareNode) attr(name string) string {
	if n != nil {
		for _, a := range n.attrs {
			if a.Name.Local == name {
				return a.Value
			}
		}
	}
	return ""
}
func (n *compareNode) relationshipID() string {
	if n != nil {
		for _, a := range n.attrs {
			if a.Name.Local == "id" && strings.HasSuffix(a.Name.Space, "/relationships") {
				return a.Value
			}
		}
	}
	return ""
}
func (n *compareNode) plainAttr(name string) string {
	if n != nil {
		for _, a := range n.attrs {
			if a.Name.Local == name && a.Name.Space == "" {
				return a.Value
			}
		}
	}
	return ""
}
func (n *compareNode) child(name string) *compareNode {
	if n != nil {
		for _, c := range n.children {
			if c.name.Local == name {
				return c
			}
		}
	}
	return nil
}
func (n *compareNode) all(name string) []*compareNode {
	out := []*compareNode{}
	if n != nil {
		for _, c := range n.children {
			if c.name.Local == name {
				out = append(out, c)
			}
		}
	}
	return out
}
func (n *compareNode) text() string {
	if n == nil {
		return ""
	}
	var out strings.Builder
	var walk func(*compareNode)
	walk = func(node *compareNode) {
		for _, part := range node.parts {
			switch value := part.(type) {
			case string:
				out.WriteString(value)
			case *compareNode:
				walk(value)
			}
		}
	}
	walk(n)
	return out.String()
}
func (n *compareNode) descendants(name string) []*compareNode {
	var out []*compareNode
	if n != nil {
		for _, c := range n.children {
			if c.name.Local == name {
				out = append(out, c)
			}
			out = append(out, c.descendants(name)...)
		}
	}
	return out
}

type compareRelationship struct {
	target, kind string
	external     bool
}

func (p *comparePackage) relationships(part string) (map[string]compareRelationship, error) {
	rels := path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
	out := map[string]compareRelationship{}
	if p.files[rels] == nil {
		return out, nil
	}
	root, err := p.xml(rels)
	if err != nil {
		return nil, err
	}
	for _, entry := range root.all("Relationship") {
		id, target, kind := entry.attr("Id"), entry.attr("Target"), entry.attr("Type")
		if id == "" || target == "" || out[id].target != "" {
			return nil, fmt.Errorf("invalid office relationship")
		}
		external := strings.EqualFold(entry.attr("TargetMode"), "External")
		if !external {
			uri, e := url.Parse(target)
			if e != nil || uri.IsAbs() || uri.Host != "" || uri.RawQuery != "" || uri.Fragment != "" {
				return nil, fmt.Errorf("invalid office relationship URI")
			}
			target = uri.Path
			if strings.Contains(target, "\\") {
				return nil, fmt.Errorf("invalid office relationship target")
			}
			if strings.HasPrefix(target, "/") {
				target = path.Clean(strings.TrimPrefix(target, "/"))
			} else {
				target = path.Clean(path.Join(path.Dir(part), target))
			}
			if target == ".." || strings.HasPrefix(target, "../") {
				return nil, fmt.Errorf("office relationship escapes package")
			}
		}
		out[id] = compareRelationship{target, kind, external}
	}
	return out, nil
}
