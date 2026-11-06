package flowsync

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestEWSOutlookConversationsAndShapes(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	var indexName string
	if err := st.(*storage.Store).DB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='index' AND name='idx_messages_internet_message_id'`).Scan(&indexName); err != nil {
		t.Fatal("conversation Message-ID lookup index missing", err)
	}

	inbox, err := st.EnsureMailbox(ctx, u.ID, "INBOX", "")
	if err != nil {
		t.Fatal(err)
	}
	sent, err := st.EnsureMailbox(ctx, u.ID, "Sent", "")
	if err != nil {
		t.Fatal(err)
	}
	add := func(id, subject, headers, folder string, hour int) *storage.Message {
		t.Helper()
		date := fmt.Sprintf("Fri, 06 Nov 2026 %02d:00:00 +0000", hour)
		raw := []byte("From: sender@example.com\r\nMessage-ID: <" + id + ">\r\nSubject: " + subject + "\r\nDate: " + date + "\r\n" + headers + "\r\nBody")
		path, size, err := eas.ms.Deliver(u.Email, "INBOX", raw)
		if err != nil {
			t.Fatal(err)
		}
		m, err := st.InsertMessage(ctx, &storage.Message{MailboxID: folder, FilePath: path, Size: size, MessageID: "<" + id + ">", Subject: subject, DateHdr: date, InternalDate: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	root := add("root@example.com", "Project & budget", "", inbox.ID, 10)
	reply := add("reply@example.com", "Re: Project & budget", "References: <root@example.com>\r\nIn-Reply-To: <root@example.com>\r\n", sent.ID, 11)
	child := add("child@example.com", "Re: Changed subject", "References: <root@example.com>\r\n\t<reply@example.com>\r\n", inbox.ID, 12)
	parentOnly := add("parent-only@example.com", "Re: Changed again", "In-Reply-To: <child@example.com>\r\n", inbox.ID, 13)
	unrelated := add("unrelated@example.com", "Project & budget", "", inbox.ID, 14)
	copyRoot := add("root@example.com", "Project & budget", "", sent.ID, 10)
	get := func(m *storage.Message, shape string) *protocolNode {
		t.Helper()
		response, err := h.getMailItem(ctx, u, m, `<GetItem>`+shape+`</GetItem>`)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := parseProtocolXML([]byte(response))
		if err != nil {
			t.Fatal(err)
		}
		return doc
	}
	var conversationID string
	var previous []byte
	for i, m := range []*storage.Message{root, reply, child, parentOnly} {
		doc := get(m, "")
		id := nodeAttr(doc.find("ConversationId"), "Id")
		index, err := base64.StdEncoding.DecodeString(doc.find("ConversationIndex").Text)
		if err != nil || len(index) != 22+5*i || index[0] != 1 {
			t.Fatal("invalid conversation index", doc.render())
		}
		if i == 0 {
			conversationID = id
		} else if id != conversationID || !bytes.HasPrefix(index, previous) {
			t.Fatal("reply hierarchy or conversation lost", doc.render())
		}
		if doc.find("ConversationTopic").Text != "Project & budget" {
			t.Fatal("original topic lost", doc.render())
		}
		previous = index
	}
	if nodeAttr(get(unrelated, "").find("ConversationId"), "Id") == conversationID {
		t.Fatal("same-subject unrelated mail merged")
	}
	if nodeAttr(get(copyRoot, "").find("ConversationId"), "Id") != conversationID {
		t.Fatal("Sent/Inbox copies split")
	}
	only := get(child, `<ItemShape><BaseShape>IdOnly</BaseShape><AdditionalProperties><FieldURI FieldURI="item:ConversationId"/></AdditionalProperties></ItemShape>`)
	if only.find("ConversationId") == nil || only.find("ConversationIndex") != nil || only.find("ConversationTopic") != nil || len(only.find("Message").Children) != 2 {
		t.Fatal("ItemShape ignored", only.render())
	}
	noConversations := get(child, `<ItemShape><BaseShape>IdOnly</BaseShape><AdditionalProperties><FieldURI FieldURI="item:Subject"/></AdditionalProperties></ItemShape>`)
	if noConversations.find("ConversationId") != nil {
		t.Fatal("extra metadata on Thunderbird's ordinary shape")
	}
	listing, err := h.findMailItems(ctx, u, inbox.ID, `<FindItem><ItemShape><BaseShape>AllProperties</BaseShape></ItemShape></FindItem>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(listing, "<t:ConversationId ") != 4 {
		t.Fatal("FindItem omitted conversation properties", listing)
	}
	listing, err = h.findMailItems(ctx, u, inbox.ID, `<SyncFolderItems><ItemShape><BaseShape>IdOnly</BaseShape></ItemShape></SyncFolderItems>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(listing, "ConversationId") {
		t.Fatal("forced conversation scan during ordinary sync")
	}
	// Foreign users may quote a known Message-ID, but must not learn its subject.
	bob := *u
	bob.ID = ""
	bob.LocalPart = "bob"
	bob.Email = "bob@example.com"
	other, err := st.CreateUser(ctx, &bob)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := st.MessagesByInternetMessageIDs(ctx, other.ID, []string{"<root@example.com>"})
	if err != nil || len(foreign) != 0 {
		t.Fatal("foreign mailbox lookup leaked", err)
	}
	fields, err := h.ewsConversationFields(ctx, other, &storage.Message{ID: "other", InternalDate: time.Now()}, messageContent{MessageID: "<other@example.com>", References: "<root@example.com>", Subject: "Other topic"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fields, conversationID) || strings.Contains(fields, "Project") {
		t.Fatal("foreign conversation data leaked", fields)
	}
}

func TestEWSOutlookOriginalThreadIndexAndOrphans(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	header := make([]byte, 22)
	header[0] = 1
	copy(header[6:], []byte("original-thread!"))
	reply := append(append([]byte(nil), header...), []byte{0, 0, 0, 1, 2}...)
	var id string
	for i, index := range [][]byte{header, reply} {
		fields, err := h.ewsConversationFields(ctx, u, &storage.Message{ID: fmt.Sprint(i)}, messageContent{MessageID: fmt.Sprintf("<%d@example.com>", i), ThreadIndex: base64.StdEncoding.EncodeToString(index), ThreadTopic: "External topic"})
		if err != nil {
			t.Fatal(err)
		}
		doc, err := parseProtocolXML([]byte(`<root xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">` + fields + `</root>`))
		if err != nil {
			t.Fatal(err)
		}
		current := nodeAttr(doc.find("ConversationId"), "Id")
		if i == 0 {
			id = current
		} else if current != id {
			t.Fatal("original Thread-Index GUID ignored")
		}
		if doc.find("ConversationIndex").Text != base64.StdEncoding.EncodeToString(index) {
			t.Fatal("sender's Thread-Index changed")
		}
	}
	fields, err := h.ewsConversationFields(ctx, u, &storage.Message{ID: "orphan"}, messageContent{MessageID: "<orphan@example.com>", References: "<missing@example.com>", ThreadIndex: "malformed", Subject: "Re: Orphan"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fields, "ConversationId") || !strings.Contains(fields, ">Orphan<") {
		t.Fatal("missing ancestor or malformed index rejected readable mail", fields)
	}
}

func TestEWSStructuredOutgoingConversationHeaders(t *testing.T) {
	index := make([]byte, 27)
	index[0] = 1
	encoded := base64.StdEncoding.EncodeToString(index)
	wire := `<Message><Subject>Re: Project</Subject><References>&lt;root@example.com&gt;</References><ConversationIndex>` + encoded + `</ConversationIndex><ConversationTopic>Project &amp; budget</ConversationTopic><InternetMessageHeaders><InternetMessageHeader HeaderName="In-Reply-To">&lt;root@example.com&gt;</InternetMessageHeader><InternetMessageHeader HeaderName="X-Injected">ignored</InternetMessageHeader></InternetMessageHeaders><Body>Reply</Body></Message>`
	doc, err := parseProtocolXML([]byte(wire))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := messageFromEWS(&storage.User{Email: "alice@example.com"}, doc.find("Message"))
	if err != nil {
		t.Fatal(err)
	}
	content, err := parseMessageContent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if content.References != "<root@example.com>" || content.InReplyTo != "<root@example.com>" || content.ThreadIndex != encoded || content.ThreadTopic != "Project & budget" || bytes.Contains(raw, []byte("X-Injected:")) {
		t.Fatal("outgoing conversation headers lost", string(raw))
	}
	doc, err = parseProtocolXML([]byte(strings.Replace(wire, "Project &amp; budget", "Project&#13;&#10;X-Injected: yes", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = messageFromEWS(&storage.User{Email: "alice@example.com"}, doc.find("Message")); err == nil {
		t.Fatal("structured thread topic allowed header injection")
	}
}
