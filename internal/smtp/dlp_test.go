package smtp

import (
	"context"
	"errors"
	"net/textproto"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestDLPRequiresTLSBeforeEnvelope(t *testing.T) {
	r := newDLPRelay(config.SMTPDLPConfig{Enabled: true, Host: smtpFixture(t, false)}, "mail.example.com", time.Second)
	trace := &deliveryTrace{}
	err := r.SendTrace("sender@example.com", []string{"recipient@example.net"}, []byte("Subject: private\r\n\r\nbody"), trace)
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatal(err)
	}
	if strings.Contains(trace.summary(), "MAIL FROM") || strings.Contains(trace.summary(), "private") {
		t.Fatal("message leaked before mandatory TLS", trace.summary())
	}
}

func TestDLPDeliveryAndRejection(t *testing.T) {
	for _, reject := range []bool{false, true} {
		r := newDLPRelay(config.SMTPDLPConfig{Enabled: true, Host: smtpFixture(t, reject), AllowPlaintext: true}, "mail.example.com", time.Second)
		trace := &deliveryTrace{}
		err := r.SendTrace("sender@example.com", []string{"recipient@example.net"}, []byte("Subject: private\r\n\r\nbody"), trace)
		var reply *textproto.Error
		if reject {
			if !errors.As(err, &reply) || reply.Code != 550 {
				t.Fatal(err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(trace.summary(), "DLP SMTP gateway") || !strings.Contains(trace.summary(), "EHLO mail.example.com") {
			t.Fatal(trace.summary())
		}
	}
}

func TestDLPOutageKeepsMailQueuedWithoutDirectFallback(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Hostname = "mail.example.com"
	cfg.SMTP.OutboundDirect = true
	cfg.SMTP.DLP = config.SMTPDLPConfig{Enabled: true, Host: smtpFixture(t, false)} // no TLS offered
	s := &Server{cfg: cfg}
	sender, _, err := s.outbound()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sender.(*outboundRelay); !ok {
		t.Fatal("DLP bypassed by direct sender")
	}
	ctx := context.Background()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	q := newOutboundQueue(st, sender, nil, nil, "mail.example.com", QueueConfig{Enabled: true, MaxAttempts: 3}, nil)
	if err = q.Enqueue(ctx, "sender@example.com", []string{"recipient@example.net"}, []byte("Subject: fixture\r\n\r\ntest"), "<test@example.com>"); err != nil {
		t.Fatal(err)
	}
	items, err := st.ClaimOutboundDue(ctx, 1)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	q.process(ctx, items[0])
	if count, err := st.CountOutbound(ctx); err != nil || count != 1 {
		t.Fatal("blocked message lost from queue", count, err)
	}
}
