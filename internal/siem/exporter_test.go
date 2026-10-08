package siem

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestExporterUDPCEF(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	addr := pc.LocalAddr().String()

	exp, err := New(Config{
		Enabled:  true,
		Protocol: "udp",
		Address:  addr,
		Facility: "local0",
		Format:   "cef",
		Vendor:   "Tayga",
		Product:  "TaygaMail",
		Version:  "test",
		Hostname: "testhost",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 4096)
		_ = pc.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			done <- ""
			return
		}
		done <- string(buf[:n])
	}()

	exp.EmitAuth(true, "u@ex.com", "1.2.3.4", "web")
	msg := <-done
	if msg == "" {
		t.Fatal("no syslog packet received")
	}
	if !strings.Contains(msg, "CEF:0|Tayga|TaygaMail|test|auth:login|") {
		t.Fatalf("unexpected payload: %s", msg)
	}
	if !strings.Contains(msg, "suser=u@ex.com") {
		t.Fatalf("missing suser: %s", msg)
	}
}
