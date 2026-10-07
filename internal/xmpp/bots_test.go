package xmpp

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"testing"
)

func TestComponentHandshakeDigest(t *testing.T) {
	id, secret := "stream123", "s3cret"
	sum := sha1.Sum([]byte(id + secret))
	want := hex.EncodeToString(sum[:])
	if got := ComponentHandshakeDigest(id, secret); got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestBotTokenSendInbox(t *testing.T) {
	srv, store, u := testServer(t)
	ctx := context.Background()

	info, err := srv.CreateBot(ctx, u.ID, "helpdesk", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Token == "" || !hasPrefix(info.Token, "tbot_") {
		t.Fatalf("token=%q", info.Token)
	}

	got, err := srv.LookupBotToken(ctx, info.Token)
	if err != nil || got.ID != info.ID {
		t.Fatalf("lookup=%+v err=%v", got, err)
	}

	// inbound to bot mailbox
	_ = srv.enqueueBotInbox(ctx, u.Email, "bob@example.com/phone",
		`<message from='bob@example.com/phone' to='alice@example.com'><body>ping</body></message>`, "ping")
	msgs, err := srv.PollBotInbox(ctx, info.ID, 10)
	if err != nil || len(msgs) != 1 || msgs[0].Body != "ping" {
		t.Fatalf("inbox=%+v err=%v", msgs, err)
	}
	msgs, _ = srv.PollBotInbox(ctx, info.ID, 10)
	if len(msgs) != 0 {
		t.Fatalf("expected empty after poll, got %+v", msgs)
	}

	if err := srv.SendBotMessage(ctx, u.Email, "bob@example.com", "hello", "chat"); err != nil {
		t.Fatal(err)
	}
	rows, err := srv.queryMAM(ctx, u.Email, "bob@example.com", 10)
	if err != nil || len(rows) == 0 {
		t.Fatalf("mam after send=%+v err=%v", rows, err)
	}

	list, err := srv.ListBots(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	_ = srv.DeleteBot(ctx, info.ID)
	list, _ = srv.ListBots(ctx)
	if len(list) != 0 {
		t.Fatalf("after delete %+v", list)
	}
	_ = store
}

func hasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}

func TestRewriteComponentFrom(t *testing.T) {
	if got := rewriteComponentFrom("echo@other/x", "bots.example.com"); got != "echo@bots.example.com/x" {
		t.Fatalf("got %s", got)
	}
	if got := rewriteComponentFrom("", "bots.example.com"); got != "bots.example.com" {
		t.Fatalf("got %s", got)
	}
}

func TestExtractBody(t *testing.T) {
	if got := extractBody(`<body>hi &amp;</body>`); got != "hi &amp;" {
		t.Fatalf("got %q", got)
	}
}
