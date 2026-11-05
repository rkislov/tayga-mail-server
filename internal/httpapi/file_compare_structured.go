package httpapi

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type structuredEntry struct {
	group, section, address, display, key, sortKey string
	index                                          int
}

var compareNumericValue = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

type structuredDocument struct {
	scope   string
	entries map[string]structuredEntry
	bytes   int
	details bool
}
type structuredChange struct {
	Kind    string `json:"kind"`
	Group   string `json:"group"`
	Section string `json:"section"`
	Address string `json:"address,omitempty"`
	Index   int    `json:"index,omitempty"`
	Before  string `json:"before"`
	After   string `json:"after"`
}

func structuredCompareFormat(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".xlsx", ".ods", ".pptx", ".odp":
		return true
	}
	return false
}
func (d *structuredDocument) add(id string, item structuredEntry) error {
	if _, ok := d.entries[id]; ok {
		return fmt.Errorf("duplicate document item")
	}
	d.bytes += len(item.display) + len(item.key) + len(item.group) + len(item.address)
	limit, bytesLimit := maxStructuredEntries, 2<<20
	if d.details {
		limit, bytesLimit = 30000, 16<<20
	}
	if len(d.entries) >= limit || d.bytes > bytesLimit {
		if d.details {
			return fmt.Errorf("document exceeds detailed comparison limit (30000 items, 16 MiB)")
		}
		return fmt.Errorf("document exceeds structured comparison limit (10000 items, 2 MiB)")
	}
	d.entries[id] = item
	return nil
}
func compareStructuredDocument(file *os.File, name string, details ...bool) (*structuredDocument, error) {
	pkg, err := openComparePackage(file)
	if err != nil {
		return nil, err
	}
	doc := &structuredDocument{entries: map[string]structuredEntry{}}
	doc.details = len(details) > 0 && details[0]
	switch strings.ToLower(filepath.Ext(name)) {
	case ".xlsx":
		doc.scope = "spreadsheet"
		err = readCompareXLSX(pkg, doc)
	case ".ods":
		doc.scope = "spreadsheet"
		err = readCompareODS(pkg, doc)
	case ".pptx":
		doc.scope = "presentation"
		err = readComparePPTX(pkg, doc)
	case ".odp":
		doc.scope = "presentation"
		err = readCompareODP(pkg, doc)
	default:
		return nil, fmt.Errorf("structured comparison format unsupported")
	}
	if err == nil && doc.details {
		err = readComparisonDetails(pkg, doc, strings.ToLower(filepath.Ext(name)))
	}
	return doc, err
}
func structuredDocumentDiff(left, right *structuredDocument) ([]structuredChange, int, int, int) {
	keys := map[string]structuredEntry{}
	for key, item := range left.entries {
		keys[key] = item
	}
	for key, item := range right.entries {
		keys[key] = item
	}
	ids := make([]string, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := keys[ids[i]], keys[ids[j]]
		if a.sortKey == b.sortKey {
			return ids[i] < ids[j]
		}
		return a.sortKey < b.sortKey
	})
	changes := []structuredChange{}
	added, removed, modified := 0, 0, 0
	for _, id := range ids {
		a, old := left.entries[id]
		b, new := right.entries[id]
		if old && new && a.key == b.key {
			continue
		}
		item := b
		kind := "change"
		if !old {
			kind = "insert"
			added++
		} else if !new {
			kind = "delete"
			removed++
			item = a
		} else {
			modified++
		}
		changes = append(changes, structuredChange{kind, item.group, item.section, item.address, item.index, a.display, b.display})
	}
	return changes, added, removed, modified
}
func compareCellAddress(row, column int) string {
	var out string
	for column > 0 {
		column--
		out = string(rune('A'+column%26)) + out
		column /= 26
	}
	return out + strconv.Itoa(row)
}
func parseCompareAddress(address string) (int, int, error) {
	column, i := 0, 0
	for i < len(address) && address[i] >= 'A' && address[i] <= 'Z' {
		column = column*26 + int(address[i]-'A'+1)
		i++
		if column > 16384 {
			return 0, 0, fmt.Errorf("cell column exceeds limit")
		}
	}
	if i == 0 || i == len(address) {
		return 0, 0, fmt.Errorf("invalid cell address")
	}
	row, err := strconv.Atoi(address[i:])
	if err != nil || row < 1 || row > 1048576 {
		return 0, 0, fmt.Errorf("cell row exceeds limit")
	}
	return row, column, nil
}
func compareCell(doc *structuredDocument, sheet string, index, row, column int, kind, value, formula, format string) error {
	if row < 1 || row > 1048576 || column < 1 || column > 16384 {
		return fmt.Errorf("cell address exceeds limit")
	}
	if formula == "" && value == "" {
		return nil
	} // Style-only empty cells do not add content.
	address := compareCellAddress(row, column)
	canonical := value
	if (kind == "number" || kind == "percentage" || kind == "currency") && formula == "" {
		numeric, unit := value, ""
		if kind == "currency" {
			if split := strings.LastIndex(value, " "); split >= 0 {
				numeric, unit = value[:split], value[split:]
			}
		}
		if len(numeric) > 128 || !compareNumericValue.MatchString(numeric) {
			return fmt.Errorf("invalid or oversized numeric cell")
		}
		if e := strings.IndexAny(numeric, "eE"); e >= 0 {
			exponent, err := strconv.Atoi(numeric[e+1:])
			if err != nil || exponent < -400 || exponent > 400 {
				return fmt.Errorf("numeric cell exponent exceeds limit")
			}
		}
		number, ok := new(big.Rat).SetString(numeric)
		if !ok {
			return fmt.Errorf("invalid numeric cell")
		}
		canonical = number.RatString() + unit
	}
	display := value
	if formula != "" {
		display = "=" + formula
		canonical = "formula:" + formula
		kind = "formula"
	} else if kind != "string" {
		display = kind + ": " + value
	}
	if format != "" {
		display += " [" + format + "]"
	}
	key := kind + "\x00" + canonical + "\x00" + format
	return doc.add("cell\x00"+sheet+"\x00"+address, structuredEntry{sheet, "cell", address, display, key, fmt.Sprintf("%04d:1:%07d:%05d", index, row, column), index})
}
func compareSheet(doc *structuredDocument, name, state string, index int) error {
	if name == "" || index > 100 {
		return fmt.Errorf("invalid or too many sheets")
	}
	display := fmt.Sprintf("%d · %s", index, state)
	return doc.add("sheet\x00"+name, structuredEntry{name, "sheet", "", display, display, fmt.Sprintf("%04d:0", index), index})
}
func compareSlide(doc *structuredDocument, id, title, text string, images []string, index int, hidden bool) error {
	if id == "" || index > 500 {
		return fmt.Errorf("invalid or too many slides")
	}
	if title == "" {
		title = strconv.Itoa(index)
	}
	base := fmt.Sprintf("%04d", index)
	display := fmt.Sprintf("%d · hidden=%t · %s", index, hidden, title)
	if err := doc.add("slide\x00"+id, structuredEntry{title, "slide", "", display, display, base + ":0", index}); err != nil {
		return err
	}
	if text != "" {
		if err := doc.add("slide-text\x00"+id, structuredEntry{title, "slide-text", "", text, text, base + ":1", index}); err != nil {
			return err
		}
	}
	sort.Strings(images)
	imageList := strings.Join(images, "\n")
	if imageList != "" {
		return doc.add("slide-images\x00"+id, structuredEntry{title, "slide-images", "", imageList, imageList, base + ":2", index})
	}
	return nil
}
func compareImage(pkg *comparePackage, target string) (string, error) {
	data, err := pkg.read(target)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	// IDs/ZIP part names do not affect image identity. Full digests avoid collisions.
	return fmt.Sprintf("%x", sum[:]), nil
}
