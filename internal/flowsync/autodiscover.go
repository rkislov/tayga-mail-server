package flowsync

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

// Autodiscover returns FlowSync + IMAP/SMTP endpoints for clients that speak
// Outlook POX, ActiveSync JSON, and Mozilla autoconfig dialects.
type autodiscover struct {
	publicURL string
	hostname  string
	cfg       *config.Config
	store     storage.Driver
}

type protoEndpoints struct {
	SSL       bool
	Base      string
	IMAPHost  string
	IMAPPort  int
	IMAPSPort int
	POPHost   string
	POPPort   int
	POP3SPort int
	SMTPHost  string
	SMTPPort  int // submission
	SMTPSPort int
	MXPort    int
	SievePort int
}

func (a *autodiscover) endpoints() protoEndpoints {
	ssl := strings.HasPrefix(strings.ToLower(a.publicURL), "https://")
	ep := protoEndpoints{
		SSL:       ssl,
		Base:      strings.TrimRight(a.publicURL, "/"),
		IMAPHost:  a.hostname,
		IMAPPort:  143,
		IMAPSPort: 993,
		POPHost:   a.hostname,
		POPPort:   110,
		POP3SPort: 995,
		SMTPHost:  a.hostname,
		SMTPPort:  587,
		SMTPSPort: 465,
		MXPort:    25,
		SievePort: 4190,
	}
	if a.cfg != nil {
		ep.IMAPPort = listenPort(a.cfg.IMAP.Listen, 143)
		ep.IMAPSPort = listenPort(a.cfg.IMAP.IMAPS, 993)
		ep.POPPort = listenPort(a.cfg.POP3.Listen, 110)
		ep.POP3SPort = listenPort(a.cfg.POP3.POP3S, 995)
		ep.SMTPPort = listenPort(a.cfg.SMTP.Submission, 587)
		ep.SMTPSPort = listenPort(a.cfg.SMTP.SMTPS, 465)
		ep.MXPort = listenPort(a.cfg.SMTP.MX, 25)
		ep.SievePort = listenPort(a.cfg.ManageSieve.Listen, 4190)
	}
	return ep
}

func listenPort(addr string, fallback int) int {
	if addr == "" {
		return fallback
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		// ":993" style
		if strings.HasPrefix(addr, ":") {
			if n, e := strconv.Atoi(addr[1:]); e == nil {
				return n
			}
		}
		return fallback
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return fallback
	}
	return n
}

func (a *autodiscover) resolveEmail(body string, r *http.Request) (email, domain string, known bool) {
	email = extractEmailFromPOX(body)
	if email == "" {
		email = r.URL.Query().Get("Email")
	}
	if email == "" {
		email = r.URL.Query().Get("email")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if at := strings.LastIndex(email, "@"); at > 0 {
		domain = email[at+1:]
	}
	if a.store != nil && domain != "" {
		if _, err := a.store.GetDomainByName(r.Context(), domain); err == nil {
			known = true
		}
	} else if domain != "" {
		known = true // no store: accept any
	}
	return email, domain, known
}

func (a *autodiscover) handlePOX(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	email, domain, known := a.resolveEmail(string(body), r)
	ep := a.endpoints()
	sslOnOff := "Off"
	if ep.SSL {
		sslOnOff = "On"
	}
	loginName := email
	if loginName == "" {
		loginName = "user@" + domain
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">`)
	b.WriteString(`<Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/outlook/responseschema/2006a">`)
	if !known && domain != "" {
		fmt.Fprintf(&b, `<Error Time="0" Id="1"><ErrorCode>600</ErrorCode><Message>Domain not hosted</Message><DebugData>%s</DebugData></Error>`, xmlEscape(domain))
	} else {
		b.WriteString(`<User>`)
		fmt.Fprintf(&b, `<DisplayName>%s</DisplayName>`, xmlEscape(displayName(email)))
		fmt.Fprintf(&b, `<EMailAddress>%s</EMailAddress>`, xmlEscape(loginName))
		b.WriteString(`</User><Account><AccountType>email</AccountType><Action>settings</Action>`)

		// ActiveSync (EXPR)
		fmt.Fprintf(&b, `<Protocol><Type>EXPR</Type><Server>%s</Server><SSL>%s</SSL><AuthPackage>Basic</AuthPackage>`,
			xmlEscape(a.hostname), sslOnOff)
		fmt.Fprintf(&b, `<ASUrl>%s/Microsoft-Server-ActiveSync</ASUrl>`, xmlEscape(ep.Base))
		fmt.Fprintf(&b, `<EwsUrl>%s/EWS/Exchange.asmx</EwsUrl>`, xmlEscape(ep.Base))
		fmt.Fprintf(&b, `<OOFUrl>%s/EWS/Exchange.asmx</OOFUrl>`, xmlEscape(ep.Base))
		b.WriteString(`</Protocol>`)

		// EWS (EXCH)
		fmt.Fprintf(&b, `<Protocol><Type>EXCH</Type><Server>%s</Server><SSL>%s</SSL><AuthPackage>Basic</AuthPackage>`,
			xmlEscape(a.hostname), sslOnOff)
		fmt.Fprintf(&b, `<EwsUrl>%s/EWS/Exchange.asmx</EwsUrl>`, xmlEscape(ep.Base))
		fmt.Fprintf(&b, `<OOFUrl>%s/EWS/Exchange.asmx</OOFUrl>`, xmlEscape(ep.Base))
		b.WriteString(`</Protocol>`)

		// IMAP
		fmt.Fprintf(&b, `<Protocol><Type>IMAP</Type><Server>%s</Server><Port>%d</Port><DomainRequired>off</DomainRequired>`+
			`<LoginName>%s</LoginName><SPA>off</SPA><SSL>%s</SSL><AuthRequired>on</AuthRequired></Protocol>`,
			xmlEscape(ep.IMAPHost), preferSecurePort(ep.SSL, ep.IMAPSPort, ep.IMAPPort), xmlEscape(loginName), sslOnOff)

		// POP3
		fmt.Fprintf(&b, `<Protocol><Type>POP3</Type><Server>%s</Server><Port>%d</Port><DomainRequired>off</DomainRequired>`+
			`<LoginName>%s</LoginName><SPA>off</SPA><SSL>%s</SSL><AuthRequired>on</AuthRequired></Protocol>`,
			xmlEscape(ep.POPHost), preferSecurePort(ep.SSL, ep.POP3SPort, ep.POPPort), xmlEscape(loginName), sslOnOff)

		// SMTP submission
		fmt.Fprintf(&b, `<Protocol><Type>SMTP</Type><Server>%s</Server><Port>%d</Port><DomainRequired>off</DomainRequired>`+
			`<LoginName>%s</LoginName><SPA>off</SPA><SSL>%s</SSL><AuthRequired>on</AuthRequired><UsePOPAuth>off</UsePOPAuth><SMTPLast>on</SMTPLast></Protocol>`,
			xmlEscape(ep.SMTPHost), preferSecurePort(ep.SSL, ep.SMTPSPort, ep.SMTPPort), xmlEscape(loginName), sslOnOff)

		b.WriteString(`</Account>`)
	}
	b.WriteString(`</Response></Autodiscover>`)

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("X-FlowSync", "Tayga-Proprietary")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b.String()))
}

func preferSecurePort(ssl bool, secure, plain int) int {
	if ssl && secure > 0 {
		return secure
	}
	if plain > 0 {
		return plain
	}
	return secure
}

func displayName(email string) string {
	if email == "" {
		return "Tayga Mail"
	}
	local, _, _ := strings.Cut(email, "@")
	if local == "" {
		return email
	}
	return local
}

func (a *autodiscover) handleJSON(w http.ResponseWriter, r *http.Request) {
	email, domain, known := a.resolveEmail("", r)
	// Path may contain email: /autodiscover/autodiscover.json/v1.0/user@ex.com?...
	if email == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i := len(parts) - 1; i >= 0; i-- {
			if strings.Contains(parts[i], "@") {
				email = strings.ToLower(parts[i])
				if at := strings.LastIndex(email, "@"); at > 0 {
					domain = email[at+1:]
				}
				if a.store != nil && domain != "" {
					_, err := a.store.GetDomainByName(r.Context(), domain)
					known = err == nil
				} else {
					known = domain != ""
				}
				break
			}
		}
	}
	ep := a.endpoints()
	if domain != "" && !known {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"ErrorCode": "InvalidUser", "ErrorMessage": "Domain not hosted"})
		return
	}
	out := map[string]any{
		"Protocol":        "ActiveSync",
		"Url":             ep.Base + "/Microsoft-Server-ActiveSync",
		"EwsUrl":          ep.Base + "/EWS/Exchange.asmx",
		"UserDisplayName": displayName(email),
		"UserEmail":       email,
		"ServerVersion":   "FlowSync/1.0",
		"X-FlowSync":      "proprietary",
		"Settings": map[string]any{
			"IMAP": map[string]any{
				"Server": ep.IMAPHost,
				"Port":   preferSecurePort(ep.SSL, ep.IMAPSPort, ep.IMAPPort),
				"SSL":    ep.SSL,
			},
			"SMTP": map[string]any{
				"Server": ep.SMTPHost,
				"Port":   preferSecurePort(ep.SSL, ep.SMTPSPort, ep.SMTPPort),
				"SSL":    ep.SSL,
			},
			"POP3": map[string]any{
				"Server": ep.POPHost,
				"Port":   preferSecurePort(ep.SSL, ep.POP3SPort, ep.POPPort),
				"SSL":    ep.SSL,
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-FlowSync", "Tayga-Proprietary")
	_ = json.NewEncoder(w).Encode(out)
}

// handleMozilla serves Thunderbird/Mozilla clientConfig 1.1 XML.
func (a *autodiscover) handleMozilla(w http.ResponseWriter, r *http.Request) {
	emailaddr := r.URL.Query().Get("emailaddress")
	if emailaddr == "" {
		emailaddr = r.URL.Query().Get("email")
	}
	email, domain, known := a.resolveEmail("<EMailAddress>"+emailaddr+"</EMailAddress>", r)
	if domain == "" {
		// path: /.well-known/autoconfig/mail/config-v1.1.xml?emailaddress=
		domain = r.URL.Query().Get("domain")
	}
	ep := a.endpoints()
	if domain != "" && a.store != nil {
		if _, err := a.store.GetDomainByName(r.Context(), domain); err != nil {
			known = false
		} else {
			known = true
		}
	}
	if domain != "" && !known {
		http.NotFound(w, r)
		return
	}
	if domain == "" {
		domain = a.hostname
	}
	imapPort, imapSock := mozillaSocket(ep.SSL, ep.IMAPSPort, ep.IMAPPort)
	popPort, popSock := mozillaSocket(ep.SSL, ep.POP3SPort, ep.POPPort)
	smtpPort, smtpSock := mozillaSocket(ep.SSL, ep.SMTPSPort, ep.SMTPPort)
	login := email
	if login == "" {
		login = "%EMAILADDRESS%"
	}
	xmlBody := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<clientConfig version="1.1">
  <emailProvider id="%s">
    <domain>%s</domain>
    <displayName>Tayga Mail</displayName>
    <displayShortName>Tayga</displayShortName>
    <incomingServer type="imap">
      <hostname>%s</hostname>
      <port>%d</port>
      <socketType>%s</socketType>
      <authentication>password-cleartext</authentication>
      <username>%s</username>
    </incomingServer>
    <incomingServer type="pop3">
      <hostname>%s</hostname>
      <port>%d</port>
      <socketType>%s</socketType>
      <authentication>password-cleartext</authentication>
      <username>%s</username>
    </incomingServer>
    <outgoingServer type="smtp">
      <hostname>%s</hostname>
      <port>%d</port>
      <socketType>%s</socketType>
      <authentication>password-cleartext</authentication>
      <username>%s</username>
    </outgoingServer>
  </emailProvider>
</clientConfig>`,
		xmlEscape(domain), xmlEscape(domain),
		xmlEscape(ep.IMAPHost), imapPort, imapSock, xmlEscape(login),
		xmlEscape(ep.POPHost), popPort, popSock, xmlEscape(login),
		xmlEscape(ep.SMTPHost), smtpPort, smtpSock, xmlEscape(login),
	)
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("X-FlowSync", "Tayga-Proprietary")
	_, _ = w.Write([]byte(xmlBody))
}

func extractEmailFromPOX(body string) string {
	lower := strings.ToLower(body)
	const tag = "<emailaddress>"
	i := strings.Index(lower, tag)
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

// mozillaSocket returns port + socketType for Mozilla autoconfig.
// TLS public URL → SSL on the secure port; otherwise STARTTLS on the cleartext port.
func mozillaSocket(ssl bool, securePort, plainPort int) (int, string) {
	if ssl {
		return preferSecurePort(true, securePort, plainPort), "SSL"
	}
	return preferSecurePort(false, securePort, plainPort), "STARTTLS"
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
