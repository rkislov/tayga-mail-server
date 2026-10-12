// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestEWSMailMetadataRoundTrip(t *testing.T) {
	ctx, store, user, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: store, ms: eas.ms}
	if err := h.ensureStandardMailFolders(ctx, user); err != nil {
		t.Fatal(err)
	}
	mb, _ := store.GetMailbox(ctx, user.ID, "INBOX")
	rel, size, err := h.ms.Deliver(user.Email, "INBOX", []byte("From: bob@example.com\r\nSubject: hello\r\n\r\nbody"))
	if err != nil {
		t.Fatal(err)
	}
	msg, err := store.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: rel, Size: size, InternalDate: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	update := func(changes string) string {
		t.Helper()
		out, err := h.updateItem(ctx, user, `<UpdateItem><ItemChanges><ItemChange><ItemId Id="`+msg.ID+`"/><Updates>`+changes+`</Updates></ItemChange></ItemChanges></UpdateItem>`)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	set := func(field, value string) string {
		return `<SetItemField><FieldURI FieldURI="` + field + `"/><Message>` + value + `</Message></SetItemField>`
	}
	for _, changes := range []string{set("item:Flag", `<Flag><FlagStatus>Flagged</FlagStatus></Flag>`) + set("item:Categories", `<Categories><String>Work</String></Categories>`) + set("message:IsRead", `<IsRead>true</IsRead>`), `<DeleteItemField><ExtendedFieldURI DistinguishedPropertySetId="Common" PropertyId="34000" PropertyType="String"/></DeleteItemField>`} {
		if got := update(changes); !strings.Contains(got, "NoError") {
			t.Fatal(got)
		}
	}
	extendedURI := `<ExtendedFieldURI DistinguishedPropertySetId="Common" PropertyId="34000" PropertyType="String"/>`
	if got := update(`<SetItemField>` + extendedURI + `<Message><ExtendedProperty>` + extendedURI + `<Value>Outlook metadata</Value></ExtendedProperty></Message></SetItemField>`); !strings.Contains(got, "NoError") {
		t.Fatal(got)
	}
	saved, _ := store.GetMessageByID(ctx, msg.ID)
	if !storage.HasFlag(saved.Flags, `\Seen`) || !storage.HasFlag(saved.Flags, `\Flagged`) {
		t.Fatal(saved.Flags)
	}
	got, err := h.getMailItem(ctx, user, saved, `<GetItem/>`)
	if err != nil {
		t.Fatal(err)
	}
	requireXML(t, got)
	if !strings.Contains(got, "Work") || !strings.Contains(got, ">Flagged<") || !strings.Contains(got, "Outlook metadata") {
		t.Fatal(got)
	}
	if got := update(set("message:IsRead", `<IsRead>false</IsRead>`) + set("item:Subject", `<Subject>changed</Subject>`)); !strings.Contains(got, "ErrorInvalidPropertySet") {
		t.Fatal(got)
	}
	saved, _ = store.GetMessageByID(ctx, msg.ID)
	if !storage.HasFlag(saved.Flags, `\Seen`) {
		t.Fatal("partially applied failed update")
	}
	if got := update(`<DeleteItemField><FieldURI FieldURI="item:Flag"/></DeleteItemField><DeleteItemField><FieldURI FieldURI="item:Categories"/></DeleteItemField>`); !strings.Contains(got, "NoError") {
		t.Fatal(got)
	}
	saved, _ = store.GetMessageByID(ctx, msg.ID)
	if storage.HasFlag(saved.Flags, `\Flagged`) {
		t.Fatal("flag retained")
	}
}
