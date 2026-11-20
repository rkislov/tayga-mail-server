package flowsync

import (
	"bytes"
	"context"
	"github.com/tayga/tms/internal/storage"
	"net/mail"
	"testing"
)

func TestEWSStructuredDelegatedSendingIdentity(t *testing.T) {
	ctx, st, actor, h, _, _ := syncFixture(t)
	owner, err := st.CreateUser(ctx, &storage.User{TenantID: actor.TenantID, DomainID: actor.DomainID, LocalPart: "boss", Email: "boss@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	grant := &storage.MailboxDelegate{OwnerID: owner.ID, DelegateID: actor.ID, CanSendOnBehalf: true}
	if err = st.UpsertMailboxDelegate(ctx, grant); err != nil {
		t.Fatal(err)
	}
	calls := 0
	sender := &mailSubmission{store: st, ms: h.ms, submit: func(_ context.Context, _ *storage.User, recipients []string, raw []byte) error {
		calls++
		message, err := mail.ReadMessage(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		from, _ := message.Header.AddressList("From")
		onBehalf, _ := message.Header.AddressList("Sender")
		if len(from) != 1 || from[0].Address != owner.Email || len(onBehalf) != 1 || onBehalf[0].Address != actor.Email {
			t.Fatal(message.Header)
		}
		return nil
	}}
	build := func(senderField string) []byte {
		doc, err := parseProtocolXML([]byte(`<t:Message><t:From><t:Mailbox><t:EmailAddress>boss@example.com</t:EmailAddress></t:Mailbox></t:From>` + senderField + `<t:ToRecipients><t:Mailbox><t:EmailAddress>remote@outside.example</t:EmailAddress></t:Mailbox></t:ToRecipients><t:Body BodyType="Text">hello</t:Body></t:Message>`))
		if err != nil {
			t.Fatal(err)
		}
		raw, err := messageFromEWS(actor, doc.find("Message"))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	raw := build(`<t:Sender><t:Mailbox><t:EmailAddress>` + actor.Email + `</t:EmailAddress></t:Mailbox></t:Sender>`)
	if err = sender.send(ctx, actor, "behalf", raw, false); err != nil {
		t.Fatal(err)
	}
	if err = sender.send(ctx, actor, "as", build(""), false); err == nil {
		t.Fatal("on behalf grant permitted send as")
	}
	if err = st.DeleteMailboxDelegate(ctx, owner.ID, actor.ID); err != nil {
		t.Fatal(err)
	}
	if err = sender.send(ctx, actor, "revoked", raw, false); err == nil {
		t.Fatal("revoked grant sent")
	}
	if calls != 1 {
		t.Fatal(calls)
	}
	// Malformed structured sender fields must be rejected without panicking.
	for _, body := range []string{`<Message><From/></Message>`, `<Message><Sender/></Message>`} {
		doc, _ := parseProtocolXML([]byte(body))
		if _, err = messageFromEWS(actor, doc.find("Message")); err == nil {
			t.Fatal(body)
		}
	}
}
