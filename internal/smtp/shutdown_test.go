package smtp

import (
	"bufio"
	"context"
	"errors"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gosmtp "github.com/emersion/go-smtp"
)

// Main and the cancellation watcher both request shutdown after SIGTERM. The
// upstream SMTP server closes a channel and must only be shut down once.
func TestConcurrentSMTPShutdown(t *testing.T) {
	old := runtime.GOMAXPROCS(8)
	defer runtime.GOMAXPROCS(old)
	var failures atomic.Int64
	for attempt := 0; attempt < 200; attempt++ {
		server := &Server{servers: []*gosmtp.Server{gosmtp.NewServer(nil), gosmtp.NewServer(nil), gosmtp.NewServer(nil)}}
		start := make(chan struct{})
		var workers sync.WaitGroup
		for n := 0; n < 16; n++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				<-start
				if err := server.Shutdown(context.Background()); err != nil {
					failures.Add(1)
				}
			}()
		}
		close(start)
		workers.Wait()
		if err := server.Shutdown(context.Background()); err != nil {
			failures.Add(1)
		}
	}
	if failures.Load() != 0 {
		t.Fatalf("concurrent/repeated shutdown errors: %d", failures.Load())
	}
}

// The first (background) watcher may wait for an active connection. Main's
// separate bounded shutdown must still return when its own deadline expires.
func TestConcurrentSMTPShutdownRespectsWaiterDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	upstream := gosmtp.NewServer(nil)
	upstream.Domain = "localhost"
	served := make(chan error, 1)
	go func() { served <- upstream.Serve(listener) }()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err = conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = bufio.NewReader(conn).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	server := &Server{servers: []*gosmtp.Server{upstream}}
	finished := make(chan error, 1)
	go func() { finished <- server.Shutdown(context.Background()) }()
	select {
	case <-served:
	case <-time.After(3 * time.Second):
		t.Fatal("listener did not stop")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err = server.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded waiter: %v", err)
	}
	conn.Close()
	select {
	case err = <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("background shutdown did not complete")
	}
}
