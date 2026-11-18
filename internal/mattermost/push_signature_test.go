package mattermost

import (
	"context"
	"testing"

	"github.com/tayga/tms/internal/chat"
)

func TestPushSigningPublicKeyPersistsAndReceiptsReturnNoContent(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	pushRegister(t, f, 0, "a")
	config := f.call(t, "GET", "/config/client", "", nil, 200)
	public := config["AsymmetricSigningPublicKey"]
	if public == nil || public == "" {
		t.Fatal("missing public signing key")
	}
	restored := New(f.store, f.api.auth, f.api.log, "")
	again, e := restored.pushPublicKey(ctx)
	if e != nil || again != public {
		t.Fatal("push signing key changed", e)
	}
	session, _ := f.store.GetOAuthTokenByAccess(ctx, f.tokens[0])
	ackID := chat.ID("test-ack")
	if e = f.api.chat.PushReceipt(ctx, ackID, session.ID, ""); e != nil {
		t.Fatal(e)
	}
	body := map[string]any{"id": ackID, "post_id": "", "type": "message", "platform": "ios", "received_at": 1}
	receipt := f.call(t, "POST", "/notifications/ack", "", body, 200)
	if len(receipt) != 1 || receipt["status"] != "OK" {
		t.Fatal("receipt disclosed extra information", receipt)
	}
	var received int64
	if e = f.store.DB().QueryRow(`SELECT received_at FROM chat_push_receipts WHERE id=?`, ackID).Scan(&received); e != nil || received <= 1 {
		t.Fatal("client receipt not persisted", received, e)
	}
	f.call(t, "POST", "/notifications/ack", "", body, 200)
	f.call(t, "POST", "/notifications/ack", "", map[string]string{"id": chat.ID("unknown-ack")}, 200)
	f.call(t, "POST", "/notifications/ack", "", map[string]string{"id": "invalid"}, 400)
	f.call(t, "GET", "/users/me", "", nil, 401)
}
