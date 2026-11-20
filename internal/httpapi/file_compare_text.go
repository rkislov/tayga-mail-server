package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxCompareFileBytes = 32 << 20
const maxCompareTextBytes = 1 << 20
const maxCompareLines = 2000

func textCompareFormat(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".md", ".csv", ".docx", ".odt":
		return true
	}
	return false
}
func officeCompareFormat(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".docx", ".doc", ".odt", ".rtf", ".txt":
		return true
	}
	return false
}

func compareDocumentText(f *os.File, name string) ([]string, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxCompareFileBytes {
		return nil, fmt.Errorf("document exceeds 32 MiB comparison limit")
	}
	var source io.Reader = f
	var member io.ReadCloser
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".docx" || ext == ".odt" {
		archive, err := zip.NewReader(f, info.Size())
		if err != nil {
			return nil, fmt.Errorf("invalid office document")
		}
		target := "word/document.xml"
		if ext == ".odt" {
			target = "content.xml"
		}
		found := false
		for _, part := range archive.File {
			if part.Name == target {
				if found || part.UncompressedSize64 > 8<<20 {
					return nil, fmt.Errorf("document text exceeds comparison limit")
				}
				found = true
				member, err = part.Open()
				if err != nil {
					return nil, err
				}
				defer member.Close()
				source = io.LimitReader(member, (8<<20)+1)
			}
		}
		if !found {
			return nil, fmt.Errorf("document body missing")
		}
	}
	var text strings.Builder
	if member == nil {
		data, err := io.ReadAll(io.LimitReader(source, maxCompareTextBytes+1))
		if err != nil {
			return nil, err
		}
		if len(data) > maxCompareTextBytes || !utf8.Valid(data) || bytes.ContainsRune(data, 0) {
			return nil, fmt.Errorf("comparison requires UTF-8 text up to 1 MiB")
		}
		text.Write(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	} else {
		decoder := xml.NewDecoder(source)
		depth := 0
		paragraph := 0
		textDepth := 0
		skipDepth := 0
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("invalid document XML")
			}
			switch node := token.(type) {
			case xml.StartElement:
				depth++
				if depth > 128 {
					return nil, fmt.Errorf("document XML too deep")
				}
				if skipDepth > 0 {
					break
				}
				ns := node.Name.Space
				word := ns == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" || ns == "http://purl.oclc.org/ooxml/wordprocessingml/main"
				odf := ns == "urn:oasis:names:tc:opendocument:xmlns:text:1.0"
				if (odf && (node.Name.Local == "tracked-changes" || node.Name.Local == "note")) || (ns == "urn:oasis:names:tc:opendocument:xmlns:office:1.0" && node.Name.Local == "annotation") {
					skipDepth = depth
					break
				}
				if (word && node.Name.Local == "p") || (odf && (node.Name.Local == "p" || node.Name.Local == "h")) {
					paragraph = depth
				}
				if word && node.Name.Local == "t" {
					textDepth = depth
				}
				if paragraph > 0 && (word || odf) {
					switch node.Name.Local {
					case "tab":
						text.WriteByte('\t')
					case "br", "line-break":
						text.WriteByte('\n')
					case "s":
						if odf {
							count := 1
							for _, attr := range node.Attr {
								if attr.Name.Local == "c" {
									var e error
									count, e = strconv.Atoi(attr.Value)
									if e != nil || count < 1 || count > maxCompareTextBytes {
										return nil, fmt.Errorf("invalid document whitespace count")
									}
								}
							}
							if text.Len()+count > maxCompareTextBytes {
								return nil, fmt.Errorf("document text exceeds 1 MiB comparison limit")
							}
							text.WriteString(strings.Repeat(" ", count))
						}
					}
				}
			case xml.CharData:
				if skipDepth == 0 && paragraph > 0 && (ext == ".odt" || textDepth > 0) {
					text.Write(node)
				}
			case xml.EndElement:
				if skipDepth > 0 {
					if depth == skipDepth {
						skipDepth = 0
					}
					depth--
					break
				}
				if paragraph == depth {
					text.WriteByte('\n')
					paragraph = 0
				}
				if textDepth == depth {
					textDepth = 0
				}
				depth--
			}
			if text.Len() > maxCompareTextBytes {
				return nil, fmt.Errorf("document text exceeds 1 MiB comparison limit")
			}
		}
	}
	content := strings.ReplaceAll(text.String(), "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return []string{}, nil
	}
	lines := strings.Split(content, "\n")
	if len(lines) > maxCompareLines {
		return nil, fmt.Errorf("document exceeds 2000 comparison lines")
	}
	return lines, nil
}

type documentDiffLine struct {
	Kind  string `json:"kind"`
	Text  string `json:"text"`
	Left  int    `json:"left,omitempty"`
	Right int    `json:"right,omitempty"`
}

// A bounded LCS compares paragraphs/lines without treating a ZIP package's
// metadata or serialization changes as edits to the document's text.
func documentLineDiff(left, right []string) ([]documentDiffLine, int, int) {
	width := len(right) + 1
	table := make([]uint16, (len(left)+1)*width)
	for i := len(left) - 1; i >= 0; i-- {
		for j := len(right) - 1; j >= 0; j-- {
			if left[i] == right[j] {
				table[i*width+j] = table[(i+1)*width+j+1] + 1
			} else {
				a, b := table[(i+1)*width+j], table[i*width+j+1]
				if a > b {
					table[i*width+j] = a
				} else {
					table[i*width+j] = b
				}
			}
		}
	}
	out := []documentDiffLine{}
	i, j, added, removed := 0, 0, 0, 0
	for i < len(left) || j < len(right) {
		if i < len(left) && j < len(right) && left[i] == right[j] {
			out = append(out, documentDiffLine{"equal", left[i], i + 1, j + 1})
			i++
			j++
		} else if i < len(left) && (j == len(right) || table[(i+1)*width+j] >= table[i*width+j+1]) {
			out = append(out, documentDiffLine{"delete", left[i], i + 1, 0})
			i++
			removed++
		} else {
			out = append(out, documentDiffLine{"insert", right[j], 0, j + 1})
			j++
			added++
		}
	}
	return out, added, removed
}
