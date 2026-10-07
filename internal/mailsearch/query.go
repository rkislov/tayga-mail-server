package mailsearch

import (
	"strings"
	"unicode"
)

// ParsedQuery is the result of parsing a user search string.
type ParsedQuery struct {
	From     string
	To       string
	Subject  string
	FreeText string // remaining tokens for FTS (AND semantics)
}

// ParseQuery understands from:/to:/subject: prefixes plus free text.
func ParseQuery(q string) ParsedQuery {
	q = strings.TrimSpace(q)
	var out ParsedQuery
	if q == "" {
		return out
	}
	tokens := tokenizeQuery(q)
	var free []string
	for _, tok := range tokens {
		lower := strings.ToLower(tok)
		switch {
		case strings.HasPrefix(lower, "from:"):
			out.From = strings.TrimSpace(tok[5:])
		case strings.HasPrefix(lower, "to:"):
			out.To = strings.TrimSpace(tok[3:])
		case strings.HasPrefix(lower, "subject:"):
			out.Subject = strings.TrimSpace(tok[8:])
		default:
			free = append(free, tok)
		}
	}
	out.FreeText = strings.Join(free, " ")
	return out
}

// FTSQuery builds an FTS5 MATCH / plainto_tsquery-friendly AND query from free text.
func (p ParsedQuery) FTSQuery() string {
	parts := strings.Fields(p.FreeText)
	if len(parts) == 0 {
		return ""
	}
	cleaned := make([]string, 0, len(parts))
	for _, w := range parts {
		w = sanitizeFTSToken(w)
		if w != "" {
			cleaned = append(cleaned, w)
		}
	}
	return strings.Join(cleaned, " ")
}

func tokenizeQuery(q string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		out = append(out, cur.String())
		cur.Reset()
	}
	for _, r := range q {
		switch {
		case r == '"':
			inQuote = !inQuote
		case unicode.IsSpace(r) && !inQuote:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

func sanitizeFTSToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '@' || r == '.' {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
