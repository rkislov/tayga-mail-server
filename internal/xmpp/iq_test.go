package xmpp

import "testing"

func TestExtractItemPayload(t *testing.T) {
	inner := `<pubsub xmlns='http://jabber.org/protocol/pubsub'><publish node='urn:xmpp:omemo:2:devices'>` +
		`<item id='current'><list xmlns='urn:xmpp:omemo:2'><device id='1'/></list></item></publish></pubsub>`
	got := extractItemPayload(inner)
	want := `<list xmlns='urn:xmpp:omemo:2'><device id='1'/></list>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParseOMEMONode(t *testing.T) {
	inner := `<pubsub xmlns='http://jabber.org/protocol/pubsub'><items node='urn:xmpp:omemo:2:bundles:42'/>`
	node := firstSubmatch(rePubsubNode, inner)
	if node != "urn:xmpp:omemo:2:bundles:42" {
		t.Fatalf("node=%q", node)
	}
}
