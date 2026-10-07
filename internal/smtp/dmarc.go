package smtp

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"net/mail"
	"net/textproto"
	"strings"

	"blitiri.com.ar/go/spf"
	"github.com/emersion/go-msgauth/dmarc"
	gosmtp "github.com/emersion/go-smtp"
	"github.com/tayga/tms/internal/scan"
)

type dmarcPolicy struct {
	action     string // tag | reject | follow
	failOpen   bool
	authservID string
	lookup     func(domain string) (*dmarc.Record, error)
	recorder   dmarcRecorder
	log        *slog.Logger
}

type dmarcRecorder interface {
	Record(ev *dmarcEvent)
}

// dmarcEvent is one inbound DMARC evaluation for aggregate reporting.
type dmarcEvent struct {
	Domain         string
	HeaderFrom     string
	SourceIP       string
	EnvelopeDomain string
	SPFResult      string // pass|fail|none
	DKIMResult     string // pass|fail|none
	Disposition    string // none|quarantine|reject
	Policy         string
	RUA            []string
}

func (p *dmarcPolicy) apply(data []byte, envelopeFrom, clientIP string, spfRes spf.Result, dkimPassDomains []string) ([]byte, error) {
	if p == nil {
		return data, nil
	}
	fromDomain := headerFromDomain(data)
	if fromDomain == "" {
		fromDomain = emailDomain(envelopeFrom)
	}
	ev := &dmarcEvent{
		Domain:         fromDomain,
		HeaderFrom:     fromDomain,
		SourceIP:       clientIP,
		EnvelopeDomain: emailDomain(envelopeFrom),
		SPFResult:      spfResultString(spfRes),
		DKIMResult:     dkimResultString(dkimPassDomains, fromDomain),
		Disposition:    "none",
		Policy:         "none",
	}
	defer func() {
		if p.recorder != nil && fromDomain != "" {
			p.recorder.Record(ev)
		}
	}()

	if fromDomain == "" {
		data = scan.InjectHeader(data, "Authentication-Results", p.authservID+"; dmarc=none")
		return data, nil
	}

	lookup := p.lookup
	if lookup == nil {
		lookup = dmarc.Lookup
	}
	rec, err := lookup(fromDomain)
	if err != nil {
		if err == dmarc.ErrNoPolicy {
			data = scan.InjectHeader(data, "Authentication-Results",
				fmt.Sprintf("%s; dmarc=none header.from=%s", p.authservID, fromDomain))
			return data, nil
		}
		if dmarc.IsTempFail(err) {
			if !p.failOpen {
				return nil, &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 7, 1}, Message: "DMARC temporary failure"}
			}
			data = scan.InjectHeader(data, "Authentication-Results",
				fmt.Sprintf("%s; dmarc=temperror header.from=%s", p.authservID, fromDomain))
			return data, nil
		}
		if p.log != nil {
			p.log.Warn("dmarc lookup", "domain", fromDomain, "err", err)
		}
		data = scan.InjectHeader(data, "Authentication-Results",
			fmt.Sprintf("%s; dmarc=permerror header.from=%s", p.authservID, fromDomain))
		return data, nil
	}

	ev.Policy = string(rec.Policy)
	if ev.Policy == "" {
		ev.Policy = "none"
	}
	ev.RUA = append([]string{}, rec.ReportURIAggregate...)

	// Only Pass counts for DMARC SPF alignment (RFC 7489).
	spfAligned := aligned(emailDomain(envelopeFrom), fromDomain, rec.SPFAlignment) && spfRes == spf.Pass

	dkimAligned := false
	for _, d := range dkimPassDomains {
		if aligned(d, fromDomain, rec.DKIMAlignment) {
			dkimAligned = true
			break
		}
	}
	pass := spfAligned || dkimAligned
	result := "fail"
	if pass {
		result = "pass"
	}
	ar := fmt.Sprintf("%s; dmarc=%s header.from=%s", p.authservID, result, fromDomain)
	if rec.Policy != "" {
		ar += " policy." + string(rec.Policy)
	}
	data = scan.InjectHeader(data, "Authentication-Results", ar)

	if pass {
		ev.Disposition = "none"
		return data, nil
	}
	policy := rec.Policy
	if policy == "" {
		policy = dmarc.PolicyNone
	}
	switch p.action {
	case "reject":
		ev.Disposition = "reject"
		return nil, &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 1}, Message: "DMARC validation failed"}
	case "follow":
		if policy == dmarc.PolicyReject {
			ev.Disposition = "reject"
			return nil, &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 1}, Message: "Message rejected by DMARC policy"}
		}
		if policy == dmarc.PolicyQuarantine {
			ev.Disposition = "quarantine"
		}
	}
	return data, nil
}

func spfResultString(r spf.Result) string {
	switch r {
	case spf.Pass:
		return "pass"
	case spf.Fail, spf.SoftFail:
		return "fail"
	case spf.None, "":
		return "none"
	default:
		return "fail"
	}
}

func dkimResultString(passDomains []string, fromDomain string) string {
	if len(passDomains) == 0 {
		return "none"
	}
	for _, d := range passDomains {
		if aligned(d, fromDomain, dmarc.AlignmentRelaxed) {
			return "pass"
		}
	}
	return "fail"
}

func headerFromDomain(data []byte) string {
	tr := textproto.NewReader(bufio.NewReader(bytes.NewReader(data)))
	hdr, err := tr.ReadMIMEHeader()
	if err != nil {
		return ""
	}
	raw := hdr.Get("From")
	if raw == "" {
		return ""
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return emailDomain(raw)
	}
	return emailDomain(addr.Address)
}

func emailDomain(addr string) string {
	addr = strings.Trim(strings.ToLower(strings.TrimSpace(addr)), "<>")
	if i := strings.LastIndex(addr, "@"); i >= 0 {
		return addr[i+1:]
	}
	return ""
}

func aligned(authDomain, fromDomain string, mode dmarc.AlignmentMode) bool {
	authDomain = strings.ToLower(strings.TrimSpace(authDomain))
	fromDomain = strings.ToLower(strings.TrimSpace(fromDomain))
	if authDomain == "" || fromDomain == "" {
		return false
	}
	if authDomain == fromDomain {
		return true
	}
	if mode == dmarc.AlignmentStrict {
		return false
	}
	return strings.HasSuffix(fromDomain, "."+authDomain) || strings.HasSuffix(authDomain, "."+fromDomain)
}
