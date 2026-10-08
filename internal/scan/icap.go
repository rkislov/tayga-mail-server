package scan

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ICAP scans messages via ICAP REQMOD (RFC 3507) — Kaspersky, Symantec, c-icap, etc.
type ICAP struct {
	rawURL  string
	timeout time.Duration
}

func NewICAP(rawURL string, timeout time.Duration) (*ICAP, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("scan.icap.url is required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("scan.icap.url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "icap", "icaps":
	default:
		return nil, fmt.Errorf("scan.icap.url: scheme must be icap:// or icaps://")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("scan.icap.url: host required")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &ICAP{rawURL: rawURL, timeout: timeout}, nil
}

func (c *ICAP) Name() string { return "icap" }

func (c *ICAP) Scan(ctx context.Context, data []byte) (*Result, error) {
	start := time.Now()
	u, err := url.Parse(c.rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	secure := strings.EqualFold(u.Scheme, "icaps")
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if secure {
			port = "11344"
		} else {
			port = "1344"
		}
	}
	addr := net.JoinHostPort(host, port)
	servicePath := u.EscapedPath()
	if servicePath == "" || servicePath == "/" {
		servicePath = "/reqmod"
	}

	d := c.timeout
	if deadline, ok := ctx.Deadline(); ok {
		if left := time.Until(deadline); left > 0 && left < d {
			d = left
		}
	}
	dialer := net.Dialer{Timeout: d}
	var conn net.Conn
	if secure {
		tlsDialer := &tls.Dialer{NetDialer: &dialer, Config: &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}}
		conn, err = tlsDialer.DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(d))

	httpHdr := "POST /mail.eml HTTP/1.1\r\n" +
		"Host: localhost\r\n" +
		"Content-Type: message/rfc822\r\n" +
		"Content-Length: " + strconv.Itoa(len(data)) + "\r\n" +
		"\r\n"
	encapOffset := len(httpHdr)

	reqURL := u.Scheme + "://" + u.Host + servicePath
	if u.RawQuery != "" {
		reqURL += "?" + u.RawQuery
	}
	var b strings.Builder
	b.WriteString("REQMOD " + reqURL + " ICAP/1.0\r\n")
	b.WriteString("Host: " + u.Host + "\r\n")
	b.WriteString("User-Agent: tayga-mail/icap\r\n")
	b.WriteString("Allow: 204\r\n")
	b.WriteString("Encapsulated: req-hdr=0, req-body=" + strconv.Itoa(encapOffset) + "\r\n")
	b.WriteString("\r\n")
	b.WriteString(httpHdr)

	if _, err := io.WriteString(conn, b.String()); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if len(data) > 0 {
		if _, err := conn.Write(data); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
	}

	br := bufio.NewReader(conn)
	statusLine, err := br.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	statusLine = strings.TrimRight(statusLine, "\r\n")
	code, reason := parseICAPStatus(statusLine)
	headers := map[string]string{}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if i := strings.IndexByte(line, ':'); i > 0 {
			k := strings.ToLower(strings.TrimSpace(line[:i]))
			v := strings.TrimSpace(line[i+1:])
			headers[k] = v
		}
	}

	res := &Result{Scanner: "icap", Duration: time.Since(start)}
	if virus := infectionFromHeaders(headers); virus != "" {
		res.Clean = false
		res.Virus = virus
		return res, nil
	}
	switch code {
	case 204:
		res.Clean = true
		return res, nil
	case 200:
		// Some gateways return 200 even when clean; infection headers already checked.
		res.Clean = true
		return res, nil
	case 403, 500, 502, 503, 504:
		if code == 403 {
			res.Clean = false
			res.Virus = reason
			if res.Virus == "" {
				res.Virus = "blocked"
			}
			return res, nil
		}
		return nil, fmt.Errorf("%w: ICAP %d %s", ErrUnavailable, code, reason)
	default:
		if code >= 400 {
			return nil, fmt.Errorf("%w: ICAP %d %s", ErrUnavailable, code, reason)
		}
		res.Clean = true
		return res, nil
	}
}

func parseICAPStatus(line string) (int, string) {
	// ICAP/1.0 200 OK
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return 0, line
	}
	code, _ := strconv.Atoi(parts[1])
	reason := ""
	if len(parts) >= 3 {
		reason = parts[2]
	}
	return code, reason
}

func infectionFromHeaders(h map[string]string) string {
	for _, key := range []string{
		"x-infection-found",
		"x-virus-name",
		"x-virus-id",
		"x-blocked",
		"x-icap-infection",
	} {
		if v := strings.TrimSpace(h[key]); v != "" {
			if key == "x-infection-found" {
				return parseInfectionFound(v)
			}
			return v
		}
	}
	return ""
}

func parseInfectionFound(v string) string {
	// Type=0; Resolution=2; Threat=Eicar-Test-Signature;
	for _, part := range strings.Split(v, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "threat=") {
			return strings.TrimSpace(part[len("threat="):])
		}
	}
	return v
}
