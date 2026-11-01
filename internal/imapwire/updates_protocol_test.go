package server_test

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-imap/backend/memory"
	server "github.com/tayga/tms/internal/imapwire"
)

type updatingBackend struct {
	backend.Backend
	updates chan backend.Update
}

func (b *updatingBackend) Updates() <-chan backend.Update { return b.updates }
func (b *updatingBackend) Login(info *imap.ConnInfo, username, password string) (backend.User, error) {
	u, err := b.Backend.Login(info, "username", password)
	return namedUser{User: u, name: username}, err
}

type namedUser struct {
	backend.User
	name string
}

func (u namedUser) Username() string { return u.name }

type wireClient struct {
	net.Conn
	r *bufio.Reader
	t *testing.T
}

func (c *wireClient) read() string {
	c.t.Helper()
	s, err := c.r.ReadString('\n')
	if err != nil {
		c.t.Fatal(err)
	}
	return strings.TrimSpace(s)
}
func (c *wireClient) write(s string) {
	c.t.Helper()
	if _, err := fmt.Fprintf(c, "%s\r\n", s); err != nil {
		c.t.Fatal(err)
	}
}
func (c *wireClient) command(s string) []string {
	c.t.Helper()
	c.write("a " + s)
	var out []string
	for {
		line := c.read()
		if strings.HasPrefix(line, "a ") {
			if !strings.HasPrefix(line, "a OK") {
				c.t.Fatal(line)
			}
			return out
		}
		out = append(out, line)
	}
}

func TestIDLEFanoutIsolationOrderAndClose(t *testing.T) {
	b := &updatingBackend{Backend: memory.New(), updates: make(chan backend.Update, 16)}
	u, err := b.Backend.Login(nil, "username", "password")
	if err != nil {
		t.Fatal(err)
	}
	if err = u.CreateMailbox("other"); err != nil {
		t.Fatal(err)
	}
	s := server.New(b)
	s.AllowInsecureAuth = true
	s.ErrorLog = log.New(io.Discard, "", 0)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	go s.Serve(l)
	connect := func(user, folder string) *wireClient {
		conn, err := net.Dial("tcp", l.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		conn.SetDeadline(time.Now().Add(10 * time.Second))
		c := &wireClient{Conn: conn, r: bufio.NewReader(conn), t: t}
		c.read()
		c.command("LOGIN " + user + " password")
		c.command("SELECT " + folder)
		return c
	}
	a, bClient := connect("username", "INBOX"), connect("username", "INBOX")
	otherFolder, otherUser := connect("username", "other"), connect("another-user", "INBOX")
	for _, c := range []*wireClient{a, bClient} {
		c.write("idle IDLE")
		if !strings.HasPrefix(c.read(), "+") {
			t.Fatal("missing IDLE continuation")
		}
	}
	post := func(update backend.Update) {
		t.Helper()
		done := update.Done()
		b.updates <- update
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("notification stuck")
		}
	}
	for _, count := range []uint32{2, 3, 4} {
		status := imap.NewMailboxStatus("INBOX", []imap.StatusItem{imap.StatusMessages})
		status.Messages = count
		post(&backend.MailboxUpdate{Update: backend.NewUpdate("username", "INBOX"), MailboxStatus: status})
		for _, c := range []*wireClient{a, bClient} {
			if got, want := c.read(), fmt.Sprintf("* %d EXISTS", count); got != want {
				t.Fatalf("%q != %q", got, want)
			}
		}
	}
	// Upstream shared one consumable FETCH channel among all connections.
	msg := imap.NewMessage(1, []imap.FetchItem{imap.FetchFlags})
	msg.Flags = []string{imap.SeenFlag}
	post(&backend.MessageUpdate{Update: backend.NewUpdate("username", "INBOX"), Message: msg})
	for _, c := range []*wireClient{a, bClient} {
		if got := c.read(); got != "* 1 FETCH (FLAGS (\\Seen))" {
			t.Fatal(got)
		}
	}
	for _, c := range []*wireClient{otherFolder, otherUser} {
		if got := c.command("NOOP"); len(got) != 0 {
			t.Fatalf("leaked another mailbox/user update: %v", got)
		}
	}
	a.Close()
	post(&backend.ExpungeUpdate{Update: backend.NewUpdate("username", "INBOX"), SeqNum: 1})
	if got := bClient.read(); got != "* 1 EXPUNGE" {
		t.Fatal(got)
	}
	bClient.write("DONE")
	if got := bClient.read(); !strings.HasPrefix(got, "idle OK") {
		t.Fatal(got)
	}
	bClient.command("SELECT other")
	post(&backend.ExpungeUpdate{Update: backend.NewUpdate("username", "INBOX"), SeqNum: 2})
	if got := bClient.command("NOOP"); len(got) != 0 {
		t.Fatalf("update crossed SELECT: %v", got)
	}
	bClient.command("LOGOUT")
}
