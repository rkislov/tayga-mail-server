package httpapi

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

const maxFileSearchText = 512 << 10

var errSearchDocument = errors.New("document text unavailable or exceeds extraction limits")

func normalizeFileSearchText(text string) string {
	return strings.Join(strings.Fields(norm.NFKC.String(text)), " ")
}
func foldFileSearchText(text string) string { return cases.Fold().String(text) }
func extractFileSearchText(ctx context.Context, f *os.File, name string) (string, error) {
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if info.Size() > maxCompareFileBytes {
		return "", errSearchDocument
	}
	var text string
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		select {
		case pdfSlots <- struct{}{}:
			defer func() { <-pdfSlots }()
		case <-ctx.Done():
			return "", ctx.Err()
		}
		extraction, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(extraction, "pdftotext", "-enc", "UTF-8", "-nopgbrk", "-", "-")
		cmd.Stdin = f
		var output boundedPDFText
		cmd.Stdout = &output
		if err = cmd.Run(); err != nil {
			return "", errSearchDocument
		}
		text = output.data.String()
	case ".docx":
		text, err = extractSearchDOCX(ctx, f, info.Size())
		if err != nil {
			return "", err
		}
	default:
		return "", errSearchDocument
	}
	if len(text) > maxFileSearchText || !utf8.ValidString(text) {
		return "", errSearchDocument
	}
	normalized := normalizeFileSearchText(text)
	if len(normalized) > maxFileSearchText {
		return "", errSearchDocument
	}
	return normalized, nil
}
func extractSearchDOCX(ctx context.Context, f *os.File, size int64) (string, error) {
	archive, err := zip.NewReader(f, size)
	if err != nil || len(archive.File) > 10000 {
		return "", errSearchDocument
	}
	parts := map[string]*zip.File{}
	total := uint64(0)
	for _, part := range archive.File {
		name := part.Name
		selected := name == "word/document.xml" || name == "word/footnotes.xml" || name == "word/endnotes.xml" || ((strings.HasPrefix(name, "word/header") || strings.HasPrefix(name, "word/footer")) && strings.HasSuffix(name, ".xml") && !strings.Contains(strings.TrimPrefix(name, "word/"), "/"))
		if !selected {
			continue
		}
		if parts[name] != nil || part.UncompressedSize64 > 8<<20 {
			return "", errSearchDocument
		}
		total += part.UncompressedSize64
		if total > 16<<20 {
			return "", errSearchDocument
		}
		parts[name] = part
	}
	if parts["word/document.xml"] == nil {
		return "", errSearchDocument
	}
	names := []string{"word/document.xml"}
	extra := []string{}
	for name := range parts {
		if name != "word/document.xml" {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	names = append(names, extra...)
	var text strings.Builder
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		part, err := parts[name].Open()
		if err != nil {
			return "", errSearchDocument
		}
		err = extractSearchWordXML(ctx, io.LimitReader(part, (8<<20)+1), &text)
		part.Close()
		if err != nil {
			return "", err
		}
		text.WriteByte('\n')
	}
	return text.String(), nil
}
func extractSearchWordXML(ctx context.Context, reader io.Reader, text *strings.Builder) error {
	decoder := xml.NewDecoder(reader)
	depth, skip, textDepth, tokens := 0, 0, 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return errSearchDocument
		}
		tokens++
		if tokens%64 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		if tokens > 1000000 {
			return errSearchDocument
		}
		switch node := token.(type) {
		case xml.StartElement:
			depth++
			if depth > 128 {
				return errSearchDocument
			}
			if skip > 0 {
				continue
			}
			word := node.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" || node.Name.Space == "http://purl.oclc.org/ooxml/wordprocessingml/main"
			if !word {
				continue
			}
			switch node.Name.Local {
			case "del", "moveFrom":
				skip = depth
			case "t":
				textDepth = depth
			case "tab", "br", "cr":
				text.WriteByte(' ')
			}
		case xml.CharData:
			if skip == 0 && textDepth > 0 {
				text.Write(node)
			}
		case xml.EndElement:
			if skip == depth {
				skip = 0
			}
			if textDepth == depth {
				textDepth = 0
			}
			if node.Name.Local == "p" || node.Name.Local == "tc" {
				text.WriteByte('\n')
			}
			depth--
		}
		if text.Len() > maxFileSearchText {
			return errSearchDocument
		}
	}
}

type fileSearchSnippet struct {
	Before string `json:"before"`
	Match  string `json:"match"`
	After  string `json:"after"`
}

func contentSearchSnippet(text, query string) (fileSearchSnippet, bool) {
	folded := foldFileSearchText(text)
	index := strings.Index(folded, query)
	if index < 0 {
		return fileSearchSnippet{}, false
	}
	runes := []rune(text)
	start, end, offset := 0, len(runes), 0
	found := false
	for i, r := range runes {
		next := offset + len(foldFileSearchText(string(r)))
		if !found && next > index {
			start = i
			found = true
		}
		if next >= index+len(query) {
			end = i + 1
			break
		}
		offset = next
	}
	left, right := start-90, end+90
	if left < 0 {
		left = 0
	}
	if right > len(runes) {
		right = len(runes)
	}
	before, after := string(runes[left:start]), string(runes[end:right])
	if left > 0 {
		before = "…" + before
	}
	if right < len(runes) {
		after += "…"
	}
	return fileSearchSnippet{Before: before, Match: string(runes[start:end]), After: after}, true
}
