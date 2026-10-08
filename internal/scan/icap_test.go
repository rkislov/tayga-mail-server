package scan

import (
	"bufio"
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestICAPCleanAndInfected(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		for i := 0; i < 2; i++ {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				br := bufio.NewReader(conn)
				reqLine, _ := br.ReadString('\n')
				if !strings.HasPrefix(reqLine, "REQMOD ") {
					return
				}
				encapBody := 0
				for {
					line, err := br.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimRight(line, "\r\n")
					if line == "" {
						break
					}
					low := strings.ToLower(line)
					if strings.HasPrefix(low, "encapsulated:") {
						// req-hdr=0, req-body=N
						if idx := strings.Index(low, "req-body="); idx >= 0 {
							encapBody, _ = strconv.Atoi(strings.TrimSpace(strings.Split(low[idx+9:], ",")[0]))
						}
					}
				}
				httpHdr := make([]byte, encapBody)
				if _, err := io.ReadFull(br, httpHdr); err != nil {
					return
				}
				// Content-Length from http headers
				cl := 0
				for _, hl := range strings.Split(string(httpHdr), "\r\n") {
					if strings.HasPrefix(strings.ToLower(hl), "content-length:") {
						cl, _ = strconv.Atoi(strings.TrimSpace(hl[15:]))
					}
				}
				body := make([]byte, cl)
				if cl > 0 {
					_, _ = io.ReadFull(br, body)
				}
				if strings.Contains(string(body), "EICAR") {
					_, _ = io.WriteString(conn, "ICAP/1.0 200 OK\r\n"+
						"X-Infection-Found: Type=0; Resolution=2; Threat=Eicar-Test-Signature;\r\n"+
						"Encapsulated: null-body=0\r\n\r\n")
					return
				}
				_, _ = io.WriteString(conn, "ICAP/1.0 204 No Content\r\n\r\n")
			}(c)
		}
	}()

	s, err := NewICAP("icap://"+ln.Addr().String()+"/avscan", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	res, err := s.Scan(ctx, []byte("From: a\r\n\r\nclean"))
	if err != nil || res == nil || !res.Clean {
		t.Fatalf("clean: %#v %v", res, err)
	}
	res, err = s.Scan(ctx, []byte("From: a\r\n\r\nEICAR-STANDARD-ANTIVIRUS-TEST-FILE"))
	if err != nil || res == nil || res.Clean || res.Virus != "Eicar-Test-Signature" {
		t.Fatalf("infected: %#v %v", res, err)
	}
}

func TestNewICAPValidation(t *testing.T) {
	if _, err := NewICAP("", time.Second); err == nil {
		t.Fatal("expected empty url error")
	}
	if _, err := NewICAP("http://x/y", time.Second); err == nil {
		t.Fatal("expected scheme error")
	}
}
