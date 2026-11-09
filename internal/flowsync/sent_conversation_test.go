package flowsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestActiveSyncSentConversationMatchesEWS(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	original := []byte("From: sender@example.com\r\nTo: alice@example.com\r\nSubject: Project\r\nMessage-ID: <root@example.com>\r\n\r\nOriginal")
	root := composeSource(t, ctx, h, u, original)
	// composeSource does not index headers; real delivery does.
	root.MessageID = "<root@example.com>"
	sent, _ := st.EnsureMailbox(ctx, u.ID, "Sent", "")
	reply := []byte("From: alice@example.com\r\nTo: sender@example.com\r\nSubject: Re: Project\r\nMessage-ID: <reply@example.com>\r\nReferences: <root@example.com>\r\nIn-Reply-To: <root@example.com>\r\n\r\nReply")
	path, size, err := h.ms.Deliver(u.Email, "Sent", reply)
	if err != nil {
		t.Fatal(err)
	}
	m, err := st.InsertMessage(ctx, &storage.Message{MailboxID: sent.ID, FilePath: path, Size: size, MessageID: "<reply@example.com>"})
	if err != nil {
		t.Fatal(err)
	}
	ews := &ewsHandler{store: st, ms: h.ms}
	var id string
	for _, folder := range []string{root.MailboxID, sent.ID} {
		response, _, err := h.syncMailCollection(ctx, u, folder, "key", false, nil)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := parseProtocolXML([]byte(response))
		if err != nil {
			t.Fatal(err)
		}
		cid := doc.find("ConversationId").Text
		if id == "" {
			id = cid
		} else if cid != id {
			t.Fatal("Inbox and Sent split")
		}
		wire, err := encodeWire(response)
		if err != nil {
			t.Fatal(err)
		}
		binary, _ := base64.StdEncoding.DecodeString(cid)
		if len(binary) != 16 || !bytes.Contains(wire, append([]byte{wbxmlOpaque, 16}, binary...)) {
			t.Fatal("ConversationId is not a 16-byte OPAQUE payload")
		}
		decoded, err := decodeWire(wire)
		if err != nil {
			t.Fatal(err)
		}
		round, err := parseProtocolXML(decoded)
		if err != nil || round.find("ConversationId").Text != cid {
			t.Fatal("binary conversation did not round trip", err)
		}
	}
	raw, _ := parseMessageContent(reply)
	c, err := ews.mailConversation(ctx, u, m, raw)
	if err != nil || c.ID != id {
		t.Fatal("EWS and ActiveSync IDs differ", err)
	}
}

func TestSmartReplyReplaceMimeKeepsLinksAndBody(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	parent := composeSource(t, ctx, h, u, []byte("From: sender@example.com\r\nMessage-ID: <parent@example.com>\r\nReferences: <root@example.com>\r\n\r\nOriginal"))
	// Includes binary bytes to prove that only headers are added.
	body := []byte{'x', 0, 0xff, '\r', '\n'}
	draft := append([]byte("From: alice@example.com\r\nTo: sender@example.com\r\nSubject: Re: Project\r\n\r\n"), body...)
	var submitted []byte
	h.sender = &mailSubmission{store: st, ms: h.ms, submit: func(_ context.Context, _ *storage.User, _ []string, raw []byte) error {
		submitted = append([]byte(nil), raw...)
		return nil
	}}
	options := fmt.Sprintf(`<SaveInSentItems/><ReplaceMime/><Source><FolderId>%s</FolderId><ItemId>%s</ItemId></Source>`, parent.MailboxID, parent.ID)
	for i := 0; i < 2; i++ {
		w, _ := composeRequest(t, h, u, "SmartReply", "sent-links", options, draft)
		if w.Code != http.StatusOK || w.Body.Len() != 0 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if !bytes.HasSuffix(submitted, body) || !bytes.Contains(submitted, []byte("In-Reply-To: <parent@example.com>")) || !bytes.Contains(submitted, []byte("References: <root@example.com> <parent@example.com>")) {
		t.Fatal("reply links or MIME bytes lost")
	}
	folder, _ := st.GetMailbox(ctx, u.ID, "Sent")
	messages, _ := st.ListMessages(ctx, folder.ID)
	if len(messages) != 1 {
		t.Fatal("retry duplicated Sent copy", len(messages))
	}
	saved, _ := h.ms.Read(messages[0].FilePath)
	if !bytes.Equal(saved, submitted) {
		t.Fatal("Sent copy differs from submitted message")
	}
}

func TestReplyHeaderPreservation(t *testing.T) {
	raw := []byte("From: alice@example.com\r\nIn-Reply-To: <existing@example.com>\r\nReferences: <existing-root@example.com>\r\n\r\nBody")
	got, err := ensureReplyHeaders(raw, msgHeaders{MessageID: "<parent@example.com>"})
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("existing links rewritten", err)
	}
	only := []byte("From: alice@example.com\r\nIn-Reply-To: <parent@example.com>\r\n\r\nBody")
	got, err = ensureReplyHeaders(only, msgHeaders{MessageID: "<parent@example.com>", References: "<root@example.com>"})
	if err != nil || !bytes.Contains(got, []byte("References: <root@example.com> <parent@example.com>")) {
		t.Fatal("partial links not completed", err)
	}
}

func TestEWSReferencedReplySaveAndAuthorization(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	parent := composeSource(t, ctx, eas, u, []byte("From: sender@example.com\r\nReply-To: replies@example.com\r\nTo: alice@example.com, colleague@example.com\r\nCc: other@example.com\r\nSubject: Project\r\nMessage-ID: <parent@example.com>\r\n\r\nOriginal"))
	h := &ewsHandler{store: st, ms: eas.ms}
	request := fmt.Sprintf(`<CreateItem MessageDisposition="SaveOnly"><Items><ReplyAllToItem><ReferenceItemId Id="%s"/><NewBodyContent BodyType="HTML">&lt;p&gt;My reply&lt;/p&gt;</NewBodyContent></ReplyAllToItem></Items></CreateItem>`, parent.ID)
	response, err := h.createMailItem(ctx, u, request)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseProtocolXML([]byte(response))
	if err != nil || doc.find("ItemId") == nil {
		t.Fatal(response, err)
	}
	draft, err := st.GetMessageByID(ctx, nodeAttr(doc.find("ItemId"), "Id"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := eas.ms.Read(draft.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	content, err := parseMessageContent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if content.InReplyTo != "<parent@example.com>" || content.References != "<parent@example.com>" || !strings.Contains(content.HTML, "My reply") || !strings.Contains(content.Text, "Original") || !strings.Contains(content.To, "colleague@example.com") || !strings.Contains(content.To, "replies@example.com") || strings.Contains(content.To, u.Email) || !strings.Contains(content.Cc, "other@example.com") {
		t.Fatal("referenced reply lost links, body or recipients", content)
	}
	response, err = h.createMailItem(ctx, &storage.User{ID: "unrelated", Email: "other@example.com"}, request)
	if err != nil || !strings.Contains(response, "ErrorItemNotFound") {
		t.Fatal("cross-owner reference accepted", response, err)
	}
}
