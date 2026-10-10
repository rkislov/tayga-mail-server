// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"strings"
	"time"
)

func changedDAVProperties(body string, fields map[string]string) map[string]bool {
	changed := map[string]bool{}
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return changed
	}
	for field, property := range fields {
		if doc.find(field) != nil {
			changed[property] = true
		}
	}
	return changed
}

// Replace only explicitly edited properties in the first component. Preserve
// timezone definitions, recurrence, alarms, photos and additional contact data.
func mergeDAVProperties(original, generated, component string, changed map[string]bool) string {
	unfold := func(raw string) []string {
		var lines []string
		for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
			if len(lines) > 0 && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
				lines[len(lines)-1] += line[1:]
			} else if line != "" {
				lines = append(lines, line)
			}
		}
		return lines
	}
	name := func(line string) string {
		return strings.ToUpper(strings.SplitN(strings.SplitN(line, ":", 2)[0], ";", 2)[0])
	}
	replacements := map[string]string{}
	for _, line := range unfold(generated) {
		if changed[name(line)] {
			replacements[name(line)] = line
		}
	}
	var out []string
	depth, completed := 0, false
	for _, line := range unfold(original) {
		if !completed && strings.EqualFold(line, "BEGIN:"+component) {
			depth = 1
			out = append(out, line)
			continue
		}
		if depth > 0 && strings.HasPrefix(strings.ToUpper(line), "BEGIN:") {
			depth++
		}
		if depth == 1 && strings.EqualFold(line, "END:"+component) {
			for property, replacement := range replacements {
				if changed[property] {
					out = append(out, replacement)
				}
			}
			completed = true
		}
		if depth == 1 && changed[name(line)] {
			if replacement := replacements[name(line)]; replacement != "" {
				out = append(out, replacement)
			}
			delete(changed, name(line))
			continue
		}
		out = append(out, line)
		if depth > 0 && strings.HasPrefix(strings.ToUpper(line), "END:") {
			depth--
		}
	}
	return strings.Join(out, "\r\n") + "\r\n"
}

func ewsDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
func davTextProperty(data, property string) string {
	for _, line := range unfoldICS(data) {
		if strings.EqualFold(strings.SplitN(strings.SplitN(line, ":", 2)[0], ";", 2)[0], property) {
			return strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`).Replace(icsValue(line))
		}
	}
	return ""
}
