package scan

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// ClamAV talks to clamd over TCP using the INSTREAM command.
type ClamAV struct {
	addr    string
	timeout time.Duration
}

func NewClamAV(addr string, timeout time.Duration) *ClamAV {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &ClamAV{addr: addr, timeout: timeout}
}

func (c *ClamAV) Name() string { return "clamav" }

func (c *ClamAV) Scan(ctx context.Context, data []byte) (*Result, error) {
	start := time.Now()
	d := c.timeout
	if deadline, ok := ctx.Deadline(); ok {
		if left := time.Until(deadline); left > 0 && left < d {
			d = left
		}
	}
	dialer := net.Dialer{Timeout: d}
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(d))

	// zINSTREAM\0 then length-prefixed chunks, ending with 0 length.
	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	const chunk = 32 << 10
	for off := 0; off < len(data); {
		n := chunk
		if off+n > len(data) {
			n = len(data) - off
		}
		var hdr [4]byte
		binary.BigEndian.PutUint32(hdr[:], uint32(n))
		if _, err := conn.Write(hdr[:]); err != nil {
			return nil, err
		}
		if _, err := conn.Write(data[off : off+n]); err != nil {
			return nil, err
		}
		off += n
	}
	var zero [4]byte
	if _, err := conn.Write(zero[:]); err != nil {
		return nil, err
	}

	resp, err := io.ReadAll(io.LimitReader(conn, 4096))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	line := strings.TrimSpace(string(bytes.TrimRight(resp, "\x00\r\n")))
	res := &Result{Scanner: "clamav", Duration: time.Since(start)}
	switch {
	case strings.HasSuffix(line, " OK") || line == "stream: OK":
		res.Clean = true
		return res, nil
	case strings.Contains(line, " FOUND"):
		res.Clean = false
		res.Virus = parseClamVirus(line)
		return res, nil
	case strings.Contains(line, "ERROR"):
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, line)
	default:
		return nil, fmt.Errorf("%w: unexpected response %q", ErrUnavailable, line)
	}
}

func parseClamVirus(line string) string {
	// stream: Eicar-Test-Signature FOUND
	line = strings.TrimPrefix(line, "stream: ")
	line = strings.TrimSuffix(line, " FOUND")
	return strings.TrimSpace(line)
}
