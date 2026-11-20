package httpapi

import (
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
	xhtml "golang.org/x/net/html"
)

// sanitizeWelcomeHTML reconstructs email markup from an allowlist. No browser
// execution context, CSS URLs, embedded documents or event handlers survive.
func sanitizeWelcomeHTML(source string) (string, error) {
	root, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		return "", err
	}
	var out strings.Builder
	allowed := "|figure|figcaption|p|div|span|br|hr|h1|h2|h3|h4|strong|b|em|i|u|s|strike|blockquote|ul|ol|li|a|img|table|thead|tbody|tfoot|tr|td|th|pre|code|"
	dropped := "|script|style|iframe|object|embed|svg|math|form|input|button|textarea|select|head|template|"
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			out.WriteString(html.EscapeString(n.Data))
			return
		}
		if n.Type != xhtml.ElementNode && n.Type != xhtml.DocumentNode {
			return
		}
		if n.Type == xhtml.ElementNode && strings.Contains(dropped, "|"+n.Data+"|") {
			return
		}
		keep := n.Type == xhtml.ElementNode && strings.Contains(allowed, "|"+n.Data+"|")
		if keep {
			out.WriteString("<" + n.Data)
			for _, a := range n.Attr {
				if a.Namespace != "" {
					continue
				}
				valid := false
				switch a.Key {
				case "href":
					valid = n.Data == "a" && welcomeURL(a.Val, false)
				case "src":
					valid = n.Data == "img" && welcomeURL(a.Val, true)
				case "class":
					valid = (n.Data == "figure" && (a.Val == "image" || a.Val == "image image_resized" || a.Val == "table"))
				case "style":
					a.Val = welcomeImageStyle(a.Val)
					valid = a.Val != ""
				case "alt", "title":
					valid = true
				case "width", "height", "colspan", "rowspan":
					valid = len(a.Val) > 0 && len(a.Val) <= 4
					for _, c := range a.Val {
						if c < '0' || c > '9' {
							valid = false
						}
					}
				case "align":
					valid = a.Val == "left" || a.Val == "center" || a.Val == "right"
				}
				if valid {
					out.WriteString(" " + a.Key + `="` + html.EscapeString(a.Val) + `"`)
				}
			}
			out.WriteString(">")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if keep && n.Data != "br" && n.Data != "hr" && n.Data != "img" {
			out.WriteString("</" + n.Data + ">")
		}
	}
	walk(root)
	return out.String(), nil
}

func welcomeURL(raw string, image bool) bool {
	if image && strings.HasPrefix(raw, "data:") {
		_, err := decodeEmbeddedMailImage(raw)
		return err == nil
	}
	if strings.ContainsAny(raw, "\x00\r\n\t{}") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme == "https" && u.Host != "" && u.User == nil {
		return true
	}
	return !image && u.Scheme == "mailto" && u.Opaque != "" && u.RawQuery == ""
}

func normalizeWelcomeContent(c *storage.WelcomeRevision) error {
	c.Subject = strings.TrimSpace(c.Subject)
	if c.Subject == "" || utf8.RuneCountInString(c.Subject) > 250 || strings.ContainsAny(c.Subject, "\r\n\x00") || len(c.HTML) > maxWelcomeHTMLBytes || utf8.RuneCountInString(c.Comment) > 1000 {
		return storage.ErrWelcomeInvalid
	}
	if _, _, err := prepareInlineMailImages(c.HTML); err != nil {
		return err
	}
	cleaned, err := sanitizeWelcomeHTML(c.HTML)
	if err != nil {
		return err
	}
	if len(cleaned) > maxWelcomeHTMLBytes {
		return storage.ErrWelcomeInvalid
	}
	// Enforce the aggregate image budget and the original text/markup budget.
	withoutData, _, err := prepareInlineMailImages(cleaned)
	if err != nil || len(withoutData) > 256<<10 {
		return storage.ErrWelcomeInvalid
	}
	c.HTML = cleaned
	c.Text = mailsearch.ExtractHTMLText(cleaned)
	if strings.TrimSpace(c.Text) == "" && !strings.Contains(cleaned, "<img ") {
		return storage.ErrWelcomeInvalid
	}
	// Only text variables are supported. URL attributes must be literal HTTPS.
	for _, v := range []string{c.Subject, c.HTML} {
		rest := v
		for {
			start := strings.Index(rest, "{{")
			if start < 0 {
				break
			}
			end := strings.Index(rest[start+2:], "}}")
			if end < 0 {
				return storage.ErrWelcomeInvalid
			}
			key := rest[start+2 : start+2+end]
			if key != "name" && key != "email" && key != "domain" && key != "tenant" {
				return storage.ErrWelcomeInvalid
			}
			rest = rest[start+2+end+2:]
		}
	}
	return nil
}

func (s *Server) handleAdminWelcome(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/welcome/"), "/")
	if len(parts) != 2 || parts[1] == "" {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	kind, id := parts[0], parts[1]
	switch kind {
	case "domains":
		d, err := s.store.GetDomainByID(r.Context(), id)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "domain not found"})
			return
		}
		if !s.adminCanManageDomain(r, admin, d.ID) {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
	case "tenants":
		if !s.isGlobalAdminUser(admin) && (!storage.HasRole(admin.Roles, storage.RoleTenantAdmin) || admin.TenantID != id) {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		if _, err := s.store.GetTenantByID(r.Context(), id); err != nil {
			writeJSON(w, 404, map[string]string{"error": "tenant not found"})
			return
		}
	default:
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		value, err := s.store.GetWelcomeTemplate(r.Context(), kind, id)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "welcome lookup failed"})
			return
		}
		// A domain admin may view the inherited published letter, never the tenant's
		// draft, audit or history. Effective is always the published revision.
		effective := value
		if kind == "domains" && value.Mode == "inherit" {
			d, err := s.store.GetDomainByID(r.Context(), id)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "welcome lookup failed"})
				return
			}
			effective, err = s.store.GetWelcomeTemplate(r.Context(), "tenants", d.TenantID)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "welcome lookup failed"})
				return
			}
		}
		var revision *storage.WelcomeRevision
		if effective.Mode == "own" {
			for _, v := range effective.Revisions {
				if v.Number == effective.Published {
					copy := v
					copy.Comment = ""
					copy.Author = ""
					revision = &copy
					break
				}
			}
		}
		stats, err := s.store.WelcomeDeliveryStats(r.Context(), kind, id)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "welcome delivery lookup failed"})
			return
		}
		writeJSON(w, 200, map[string]any{"template": value, "effective": revision, "source": effective.Kind, "source_id": effective.ScopeID, "delivery_stats": stats})
	case http.MethodPost:
		var c storage.WelcomeChange
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20))
		dec.DisallowUnknownFields()
		if dec.Decode(&c) != nil || dec.Decode(new(any)) != io.EOF {
			writeJSON(w, 400, map[string]string{"error": "invalid welcome template"})
			return
		}
		if c.Action == "save" || c.Action == "preview" {
			if err := normalizeWelcomeContent(&c.Content); err != nil {
				writeJSON(w, 400, map[string]string{"error": "use a subject, HTML body and supported variables; maximum text size is 256 KB; embedded images up to 2 MiB each and 8 MiB total"})
				return
			}
		}
		if utf8.RuneCountInString(c.Content.Comment) > 1000 {
			writeJSON(w, 400, map[string]string{"error": "comment is too long"})
			return
		}
		if c.Action == "preview" {
			writeJSON(w, 200, c.Content)
			return
		}
		value, err := s.store.ChangeWelcomeTemplate(r.Context(), kind, id, admin.Email, c)
		switch {
		case errors.Is(err, storage.ErrWelcomeConflict):
			writeJSON(w, 409, map[string]string{"error": err.Error()})
		case errors.Is(err, storage.ErrNotFound):
			writeJSON(w, 404, map[string]string{"error": "revision not found"})
		case errors.Is(err, storage.ErrWelcomeInvalid):
			writeJSON(w, 400, map[string]string{"error": "invalid action or mode; publish a revision before enabling own template"})
		case err != nil:
			writeJSON(w, 500, map[string]string{"error": "welcome save failed"})
		default:
			writeJSON(w, 200, value)
		}
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

// CKEditor emits figure widths for image resizing. Keep safe layout declarations.
func welcomeImageStyle(value string) string {
	var out []string
	for _, declaration := range strings.Split(value, ";") {
		key, val, ok := strings.Cut(declaration, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "text-align" && (val == "left" || val == "center" || val == "right") {
			out = append(out, key+":"+val)
			continue
		}
		if key != "width" && key != "height" && key != "max-width" {
			continue
		}
		digits := strings.TrimSuffix(strings.TrimSuffix(val, "px"), "%")
		if digits == val || len(digits) == 0 || len(digits) > 4 {
			continue
		}
		valid := true
		for _, r := range digits {
			if r < '0' || r > '9' {
				valid = false
			}
		}
		if valid {
			out = append(out, key+":"+val)
		}
	}
	return strings.Join(out, ";")
}
