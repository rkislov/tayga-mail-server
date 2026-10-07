package scan

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestClamAVCleanAndInfected(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 2; i++ {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			func(conn net.Conn) {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				cmd := make([]byte, 10)
				if _, err := io.ReadFull(conn, cmd); err != nil {
					return
				}
				var total int
				for {
					var hdr [4]byte
					if _, err := io.ReadFull(conn, hdr[:]); err != nil {
						return
					}
					n := int(binary.BigEndian.Uint32(hdr[:]))
					if n == 0 {
						break
					}
					chunk := make([]byte, n)
					if _, err := io.ReadFull(conn, chunk); err != nil {
						return
					}
					total += n
				}
				if total >= 100 {
					_, _ = io.WriteString(conn, "stream: Eicar-Test-Signature FOUND\x00")
				} else {
					_, _ = io.WriteString(conn, "stream: OK\x00")
				}
			}(c)
		}
	}()

	s := NewClamAV(ln.Addr().String(), 5*time.Second)
	ctx := context.Background()
	res, err := s.Scan(ctx, []byte("From: a\r\n\r\nclean"))
	if err != nil || res == nil || !res.Clean {
		t.Fatalf("clean: %#v %v", res, err)
	}
	infected := make([]byte, 150)
	for i := range infected {
		infected[i] = 'x'
	}
	res, err = s.Scan(ctx, infected)
	if err != nil || res == nil || res.Clean || res.Virus != "Eicar-Test-Signature" {
		t.Fatalf("infected: %#v %v", res, err)
	}
	<-done
}
