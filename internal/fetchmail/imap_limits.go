package fetchmail

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
)

// go-imap v1 allocates a literal before reading it and doesn't expose its
// Reader.MaxLiteralSize on the client. Validate framing before passing it to
// that parser. Literal bodies stream through unchanged (including CRLF/braces).
// Bound both individual literals and aggregate response size.
type imapLimitedConn struct {
	net.Conn
	reader   *bufio.Reader
	pending  []byte
	literal  int64
	response int64
	limit    int64
}

const imapLineLimit = 2 << 20

func limitIMAP(conn net.Conn, limit int64) *imapLimitedConn {
	return &imapLimitedConn{Conn: conn, reader: bufio.NewReader(conn), limit: limit}
}
func (c *imapLimitedConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(c.pending) > 0 {
		n := copy(p, c.pending)
		c.pending = c.pending[n:]
		return n, nil
	}
	if c.literal > 0 {
		if int64(len(p)) > c.literal {
			p = p[:c.literal]
		}
		n, e := c.reader.Read(p)
		c.literal -= int64(n)
		return n, e
	}
	var line []byte
	for {
		part, e := c.reader.ReadSlice('\n')
		if len(line)+len(part) > imapLineLimit {
			return 0, errors.New("IMAP response line exceeds limit")
		}
		line = append(line, part...)
		if e == bufio.ErrBufferFull {
			continue
		}
		if e != nil {
			return 0, e
		}
		break
	}
	c.response += int64(len(line))
	trimmed := strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r")
	if strings.HasSuffix(trimmed, "}") {
		if start := strings.LastIndexByte(trimmed, '{'); start >= 0 {
			count := strings.TrimSuffix(trimmed[start+1:len(trimmed)-1], "+")
			n, e := strconv.ParseUint(count, 10, 32)
			if e == nil {
				if int64(n) > c.limit {
					return 0, errors.New("IMAP literal exceeds message size limit")
				}
				c.literal = int64(n)
				c.response += int64(n)
			}
		}
	}
	if c.response > c.limit+imapLineLimit {
		return 0, errors.New("IMAP response exceeds size limit")
	}
	if c.literal == 0 {
		c.response = 0
	}
	c.pending = line
	return c.Read(p)
}

// Upgrade before starting the IMAP parser so the resource guard sees plaintext
// on both implicit TLS and STARTTLS connections. Discard pre-TLS capabilities;
// client.New will obtain fresh capabilities over the verified TLS connection.
func imapStartTLS(ctx context.Context, conn net.Conn, tlsOpts *tls.Config) (net.Conn, error) {
	r := bufio.NewReader(conn)
	greeting, err := popLine(r)
	if err != nil || !strings.HasPrefix(strings.ToUpper(greeting), "* OK") {
		return nil, errors.New("IMAP greeting rejected")
	}
	if _, err = io.WriteString(conn, "collector_tls STARTTLS\r\n"); err != nil {
		return nil, err
	}
	for i := 0; i < 50; i++ {
		line, e := popLine(r)
		if e != nil {
			return nil, e
		}
		if strings.HasPrefix(line, "*") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "collector_tls" || strings.ToUpper(fields[1]) != "OK" {
			return nil, errors.New("IMAP STARTTLS unavailable")
		}
		if r.Buffered() != 0 {
			return nil, errors.New("unexpected data before IMAP TLS handshake")
		}
		tc := tls.Client(conn, tlsOpts)
		if err = tc.HandshakeContext(ctx); err != nil {
			return nil, err
		}
		return &imapReplayConn{Conn: tc, reader: io.MultiReader(strings.NewReader("* OK secure connection ready\r\n"), tc)}, nil
	}
	return nil, errors.New("IMAP STARTTLS response exceeds limit")
}

type imapReplayConn struct {
	net.Conn
	reader io.Reader
}

func (c *imapReplayConn) Read(p []byte) (int, error) { return c.reader.Read(p) }
