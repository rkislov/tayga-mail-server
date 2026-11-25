package mattermost

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strconv"
)

// openAITransport keeps the request hostname and TLS verification intact while
// connecting to a local TCP relay that forwards api.openai.com:443.
func openAITransport(relay string) (*http.Transport, error) {
	host, port, err := net.SplitHostPort(relay)
	if err != nil {
		return nil, errors.New("OpenAI relay must use a loopback address and port")
	}
	ip, ipErr := netip.ParseAddr(host)
	number, portErr := strconv.ParseUint(port, 10, 16)
	if ipErr != nil || !ip.IsLoopback() || portErr != nil || number == 0 {
		return nil, errors.New("OpenAI relay must use a loopback address and port")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	dial := transport.DialContext
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "api.openai.com:443" {
			return nil, errors.New("OpenAI relay destination is not allowed")
		}
		return dial(ctx, network, relay)
	}
	return transport, nil
}
