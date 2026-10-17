package smtp

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"
	"unicode"
)

// SMTP diagnostics contain envelope commands and server responses only. DATA and
// credentials never enter the recorder. Bound hostile multi-line server replies.
type deliveryTrace struct {
	log                *slog.Logger
	queueID, messageID string
	attempt            int
	peer, stage        string
	lines              []string
	bytes              int
	auth               bool
}

func (t *deliveryTrace) record(kind, detail string) {
	if t == nil {
		return
	}
	detail = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, detail)
	if len(detail) > 2048 {
		detail = detail[:2048] + "…"
	}
	line := fmt.Sprintf("%s peer=%s stage=%s %s %s", time.Now().UTC().Format(time.RFC3339Nano), t.peer, t.stage, kind, detail)
	if t.bytes+len(line) > 32768 || len(t.lines) >= 128 {
		return
	}
	t.lines = append(t.lines, line)
	t.bytes += len(line)
	if t.log != nil {
		t.log.Info("smtp dialogue", "queue_id", t.queueID, "message_id", t.messageID, "attempt", t.attempt, "peer", t.peer, "stage", t.stage, "direction", kind, "detail", detail)
	}
}
func (t *deliveryTrace) command(stage, command string) {
	if t != nil {
		t.stage = stage
		t.record("C:", command)
	}
}
func (t *deliveryTrace) failure(err error) error {
	if err != nil && t != nil {
		t.record("error", err.Error())
		return fmt.Errorf("%s at %s: %w", t.stage, t.peer, err)
	}
	return err
}
func (t *deliveryTrace) summary() string {
	if t == nil {
		return ""
	}
	return fmt.Sprintf("queue_id=%s attempt=%d\n%s", t.queueID, t.attempt, strings.Join(t.lines, "\n"))
}

type tracedSender interface {
	SendTrace(string, []string, []byte, *deliveryTrace) error
}

func sendTraced(sender outboundSender, from string, to []string, data []byte, t *deliveryTrace) error {
	if s, ok := sender.(tracedSender); ok {
		return s.SendTrace(from, to, data, t)
	}
	return sender.Send(from, to, data)
}

type smtpReplyReader struct {
	io.Reader
	t       *deliveryTrace
	line    []byte
	enabled bool
}

func (r *smtpReplyReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if r.enabled {
		for _, b := range p[:n] {
			if b == '\n' {
				line := strings.TrimSuffix(string(r.line), "\r")
				if r.t.auth {
					code := ""
					if len(line) >= 3 {
						code = line[:3]
					}
					line = code + " [authentication response redacted]"
				}
				r.t.record("S:", line)
				r.line = r.line[:0]
			} else if len(r.line) < 2048 {
				r.line = append(r.line, b)
			}
		}
	}
	return n, err
}

type greetingConn struct {
	net.Conn
	reader *smtpReplyReader
}

func (c *greetingConn) Read(p []byte) (int, error) { return c.reader.Read(p) }
func newTracedClient(conn net.Conn, host string, t *deliveryTrace) (*smtp.Client, error) {
	if t == nil {
		return smtp.NewClient(conn, host)
	}
	t.stage = "greeting"
	reader := &smtpReplyReader{Reader: conn, t: t, enabled: true}
	c, err := smtp.NewClient(&greetingConn{conn, reader}, host)
	reader.enabled = false // TLS later reads through this connection: never log ciphertext.
	if err == nil {
		traceClientReplies(c, t)
	}
	return c, err
}
func traceClientReplies(c *smtp.Client, t *deliveryTrace) {
	if t != nil {
		c.Text.R = bufio.NewReader(&smtpReplyReader{Reader: c.Text.R, t: t, enabled: true})
	}
}

// net/smtp performs a fresh EHLO internally during STARTTLS before returning.
// It replaces Text, so reattach the plaintext reader for subsequent commands.
func traceTLSReady(c *smtp.Client, t *deliveryTrace) {
	traceClientReplies(c, t)
	if t != nil {
		state, ok := c.TLSConnectionState()
		if ok {
			t.record("tls", fmt.Sprintf("version=0x%04x cipher=0x%04x verified=%t; EHLO accepted after TLS", state.Version, state.CipherSuite, len(state.VerifiedChains) > 0))
		}
	}
}
