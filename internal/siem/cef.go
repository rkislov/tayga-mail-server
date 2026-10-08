package siem

import (
	"fmt"
	"sort"
	"strings"
)

// FormatCEF builds a CEF 0.1 line.
// CEF:Version|Device Vendor|Device Product|Device Version|Device Event Class ID|Name|Severity|Extension
func FormatCEF(vendor, product, version, signature, name string, severity int, ext map[string]string) string {
	if vendor == "" {
		vendor = "Tayga"
	}
	if product == "" {
		product = "TaygaMail"
	}
	if version == "" {
		version = "dev"
	}
	if signature == "" {
		signature = "unknown"
	}
	if name == "" {
		name = signature
	}
	if severity < 0 {
		severity = 0
	}
	if severity > 10 {
		severity = 10
	}
	var b strings.Builder
	b.WriteString("CEF:0|")
	b.WriteString(cefEscapeHeader(vendor))
	b.WriteByte('|')
	b.WriteString(cefEscapeHeader(product))
	b.WriteByte('|')
	b.WriteString(cefEscapeHeader(version))
	b.WriteByte('|')
	b.WriteString(cefEscapeHeader(signature))
	b.WriteByte('|')
	b.WriteString(cefEscapeHeader(name))
	b.WriteByte('|')
	b.WriteString(fmt.Sprintf("%d", severity))
	b.WriteByte('|')
	if len(ext) > 0 {
		keys := make([]string, 0, len(ext))
		for k := range ext {
			if strings.TrimSpace(k) == "" || strings.TrimSpace(ext[k]) == "" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(k)
			b.WriteByte('=')
			b.WriteString(cefEscapeExt(ext[k]))
		}
	}
	return b.String()
}

func cefEscapeHeader(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `|`, `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func cefEscapeExt(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `=`, `\=`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}
