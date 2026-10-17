package smtp

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

func smtpFixture(t *testing.T, reject bool) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(5 * time.Second))
		fmt.Fprint(c, "220 mx.example.com ESMTP\r\n")
		r := textproto.NewReader(bufio.NewReader(c))
		for {
			line, e := r.ReadLine()
			if e != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				fmt.Fprint(c, "250-mx.example.com\r\n250 SIZE 999999\r\n")
			case strings.HasPrefix(line, "MAIL"):
				fmt.Fprint(c, "250 2.1.0 Sender accepted\r\n")
			case strings.HasPrefix(line, "RCPT"):
				if reject {
					fmt.Fprint(c, "550 5.1.1 No such user\r\n")
				} else {
					fmt.Fprint(c, "250 2.1.5 Recipient accepted\r\n")
				}
			case line == "DATA":
				fmt.Fprint(c, "354 Send message\r\n")
				io.Copy(io.Discard, r.DotReader())
				fmt.Fprint(c, "250 2.0.0 queued as REMOTE123\r\n")
			case line == "QUIT":
				return
			default:
				fmt.Fprint(c, "500 unknown\r\n")
			}
		}
	}()
	return ln.Addr().String()
}
func TestSMTPDialogueSuccessAndPermanentFailure(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(fmt.Sprint(reject), func(t *testing.T) {
			trace := &deliveryTrace{queueID: "queue-1", messageID: "<test@example.com>", attempt: 2}
			d := newDirectSender(2*time.Second, "mail.example.com", nil, nil, nil, nil)
			d.trace = trace
			err := d.dialAndSend(smtpFixture(t, reject), "mx.example.com", "example.com", "sender@example.com", []string{"recipient@example.net"}, []byte("Subject: SECRET SUBJECT\r\n\r\nPRIVATE BODY\r\n"), false)
			if reject {
				var reply *textproto.Error
				if !errors.As(err, &reply) || reply.Code != 550 {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal("accepted DATA must not fail because QUIT disconnected:", err)
			}
			log := trace.summary()
			for _, want := range []string{"220 mx.example.com", "EHLO mail.example.com", "MAIL FROM:", "RCPT TO:", "attempt=2"} {
				if !strings.Contains(log, want) {
					t.Fatal("missing", want, log)
				}
			}
			if !reject && !strings.Contains(log, "250 2.0.0 queued as REMOTE123") {
				t.Fatal(log)
			}
			if strings.Contains(log, "SECRET SUBJECT") || strings.Contains(log, "PRIVATE BODY") {
				t.Fatal("message body leaked")
			}
		})
	}
}
func TestSMTPDialogueRedactsAuthAndBoundsReplies(t *testing.T) {
	trace := &deliveryTrace{auth: true}
	r := &smtpReplyReader{Reader: strings.NewReader("334 sensitive-challenge\r\n535 echoed-password\r\n"), t: trace, enabled: true}
	io.Copy(io.Discard, r)
	if strings.Contains(trace.summary(), "sensitive") || strings.Contains(trace.summary(), "password") {
		t.Fatal(trace.summary())
	}
	trace.auth = false
	r.Reader = strings.NewReader(strings.Repeat("X", 100000) + "\r\n")
	io.Copy(io.Discard, r)
	if len(trace.summary()) > 4000 {
		t.Fatal("unbounded server response")
	}
}

func TestSMTPDialogueAfterSTARTTLS(t *testing.T) {
	hs := httptest.NewTLSServer(nil)
	cert := hs.TLS.Certificates[0]
	roots := x509.NewCertPool()
	roots.AddCert(hs.Certificate())
	hs.Close()
	a, b := net.Pipe()
	defer a.Close()
	a.SetDeadline(time.Now().Add(5 * time.Second))
	done := make(chan error, 1)
	go func() {
		defer b.Close()
		io.WriteString(b, "220 mx.test ready\r\n")
		r := bufio.NewReader(b)
		r.ReadString('\n')
		io.WriteString(b, "250-mx.test\r\n250 STARTTLS\r\n")
		r.ReadString('\n')
		io.WriteString(b, "220 upgrade\r\n")
		tc := tls.Server(b, &tls.Config{Certificates: []tls.Certificate{cert}})
		if err := tc.Handshake(); err != nil {
			done <- err
			return
		}
		r = bufio.NewReader(tc)
		r.ReadString('\n')
		io.WriteString(tc, "250 mx.test\r\n")
		r.ReadString('\n')
		io.WriteString(tc, "250 2.1.0 TLS sender accepted\r\n")
		done <- nil
	}()
	trace := &deliveryTrace{}
	c, e := newTracedClient(a, "example.com", trace)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	trace.command("ehlo", "EHLO collector.test")
	if e = c.Hello("collector.test"); e != nil {
		t.Fatal(e)
	}
	trace.command("starttls", "STARTTLS")
	if e = c.StartTLS(&tls.Config{ServerName: "example.com", RootCAs: roots, MinVersion: tls.VersionTLS12}); e != nil {
		t.Fatal(e)
	}
	traceTLSReady(c, trace)
	trace.command("mail", "MAIL FROM:<a@example.com>")
	if e = c.Mail("a@example.com"); e != nil {
		t.Fatal(e)
	}
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	for _, want := range []string{"220 upgrade", "verified=true", "250 2.1.0 TLS sender accepted"} {
		if !strings.Contains(trace.summary(), want) {
			t.Fatal("missing", want, trace.summary())
		}
	}
}
