package mattermost

import (
	"context"
	"net"
	"testing"
)

func TestOpenAIRelayRoutesOnlyTheOfficialEndpoint(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	transport, err := openAITransport(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	local, err := transport.DialContext(t.Context(), "tcp", "api.openai.com:443")
	if err != nil {
		t.Fatal(err)
	}
	local.Close()
	if local.RemoteAddr().String() != listener.Addr().String() {
		t.Fatal("did not use relay")
	}
	if _, err := transport.DialContext(context.Background(), "tcp", "other.example:443"); err == nil {
		t.Fatal("relay allowed a different destination")
	}
	if _, err := openAITransport("192.0.2.1:18790"); err == nil {
		t.Fatal("accepted a non-loopback relay")
	}
}
