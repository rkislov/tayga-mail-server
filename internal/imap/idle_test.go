package imapserver

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
	imapserv "github.com/tayga/tms/internal/imapwire"
)

type idleBackend struct {
	backend.Backend
	updates chan backend.Update
}

func (b *idleBackend) Updates() <-chan backend.Update { return b.updates }

// Exercise the real protocol path, with updates enabled, while a client changes
// folders. In particular, SELECT and IDLE must not share mutable context with
// the notification dispatcher without synchronization.
func TestIDLEConcurrentSelect(t *testing.T) {
	be := &idleBackend{Backend: memory.New(), updates: make(chan backend.Update, 32)}
	srv := imapserv.New(be)
	srv.AllowInsecureAuth = true
	srv.ErrorLog = log.New(io.Discard, "", 0)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go srv.Serve(ln)
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(15 * time.Second))
	r := bufio.NewReader(c)
	read := func() string {
		t.Helper()
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(line)
	}
	read()
	command := func(cmd string) {
		t.Helper()
		if _, err := fmt.Fprintf(c, "a %s\r\n", cmd); err != nil {
			t.Fatal(err)
		}
		for {
			line := read()
			if strings.HasPrefix(line, "a ") {
				if !strings.HasPrefix(line, "a OK") {
					t.Fatalf("%s: %s", cmd, line)
				}
				return
			}
		}
	}
	command("LOGIN username password")
	command("CREATE other")
	stop, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			status := imap.NewMailboxStatus("INBOX", []imap.StatusItem{imap.StatusMessages})
			status.Messages = 1
			u := &backend.MailboxUpdate{Update: backend.NewUpdate("username", "INBOX"), MailboxStatus: status}
			done := u.Done()
			select {
			case be.updates <- u:
			case <-stop:
				return
			}
			select {
			case <-done:
			case <-stop:
				return
			}
		}
	}()
	defer func() { close(stop); <-stopped }()
	for i := 0; i < 100; i++ {
		command("SELECT INBOX")
		command("SELECT other")
	}
	command("LOGOUT")
}
