package flowsync

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"strings"
)

// Autodiscover returns FlowSync endpoints for clients that speak
// ActiveSync/EWS discovery dialects. Implementation is proprietary Tayga logic.
type autodiscover struct {
	publicURL string
	hostname  string
}

func (a *autodiscover) handlePOX(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	email := extractEmailFromPOX(string(body))
	if email == "" {
		email = r.URL.Query().Get("Email")
	}
	base := strings.TrimRight(a.publicURL, "/")
	resp := `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/outlook/responseschema/2006a">
    <Account>
      <AccountType>email</AccountType>
      <Action>settings</Action>
      <Protocol>
        <Type>EXPR</Type>
        <Server>` + xmlEscape(a.hostname) + `</Server>
        <SSL>Off</SSL>
        <AuthPackage>Basic</AuthPackage>
        <ASUrl>` + xmlEscape(base) + `/Microsoft-Server-ActiveSync</ASUrl>
        <EwsUrl>` + xmlEscape(base) + `/EWS/Exchange.asmx</EwsUrl>
        <OOFUrl>` + xmlEscape(base) + `/EWS/Exchange.asmx</OOFUrl>
      </Protocol>
    </Account>
  </Response>
</Autodiscover>`
	_ = email
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("X-FlowSync", "Tayga-Proprietary")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(resp))
}

func (a *autodiscover) handleJSON(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimRight(a.publicURL, "/")
	out := map[string]any{
		"Protocol":          "ActiveSync",
		"Url":               base + "/Microsoft-Server-ActiveSync",
		"EwsUrl":            base + "/EWS/Exchange.asmx",
		"UserDisplayName":   "FlowSync",
		"ServerVersion":     "FlowSync/1.0",
		"X-FlowSync-Engine": "proprietary",
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-FlowSync", "Tayga-Proprietary")
	_ = json.NewEncoder(w).Encode(out)
}

func extractEmailFromPOX(body string) string {
	const tag = "<EMailAddress>"
	i := strings.Index(strings.ToLower(body), strings.ToLower(tag))
	if i < 0 {
		return ""
	}
	rest := body[i+len(tag):]
	j := strings.Index(rest, "<")
	if j < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:j])
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
