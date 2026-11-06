package flowsync

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

type ewsMetadataCountingStore struct {
	storage.Driver
	singles, bulk int
}

func (s *ewsMetadataCountingStore) EWSMailMetadata(ctx context.Context, id string) (string, error) {
	s.singles++
	return s.Driver.EWSMailMetadata(ctx, id)
}
func (s *ewsMetadataCountingStore) EWSMailboxMetadata(ctx context.Context, id string) (map[string]string, error) {
	s.bulk++
	return s.Driver.EWSMailboxMetadata(ctx, id)
}

func TestEWSMailListingBulkMetadataAndFolderIsolation(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	inbox, err := st.EnsureMailbox(ctx, u.ID, "INBOX", "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.EnsureMailbox(ctx, u.ID, "Other", "")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i, folder := range []string{inbox.ID, inbox.ID, other.ID} {
		msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: folder, Subject: "Indexed", DateHdr: "Mon, 19 Oct 2026 12:34:56 +0300", MessageID: "<indexed@example.com>", InternalDate: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, msg.ID)
		if i != 1 {
			if err := st.UpdateEWSMailMetadata(ctx, msg.ID, `\Flagged`, `{"item:Categories":"<t:Categories><t:String>Work</t:String></t:Categories>"}`); err != nil {
				t.Fatal(err)
			}
		}
	}
	values, err := st.EWSMailboxMetadata(ctx, inbox.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[ids[0]] == "" || values[ids[2]] != "" {
		t.Fatal("metadata escaped mailbox", values)
	}
	wrapper := &ewsMetadataCountingStore{Driver: st}
	h := &ewsHandler{store: wrapper, ms: eas.ms}
	got, err := h.findMailItems(ctx, u, inbox.ID)
	if err != nil {
		t.Fatal(err)
	}
	requireXML(t, got)
	if wrapper.singles != 0 || wrapper.bulk != 1 {
		t.Fatalf("listing performs per-message SQL: single=%d bulk=%d", wrapper.singles, wrapper.bulk)
	}
	if !strings.Contains(got, ids[0]) || !strings.Contains(got, ids[1]) || strings.Contains(got, ids[2]) || !strings.Contains(got, ">Flagged<") || !strings.Contains(got, ">NotFlagged<") || !strings.Contains(got, "Work") {
		t.Fatal("bulk metadata/defaults changed", got)
	}
}

func TestEWSMetadataStopsBeforePayload(t *testing.T) {
	for _, tc := range []struct {
		name, header string
		attached     bool
	}{
		{"plain", "Subject: Fast plain\r\nContent-Type: text/plain\r\n\r\n", false},
		{"attachment", "Subject: Fast attachment\r\nContent-Type: multipart/mixed; boundary=b\r\n\r\n--b\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=huge.bin\r\n\r\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &headerCountingReader{Reader: io.MultiReader(strings.NewReader(tc.header), strings.NewReader(strings.Repeat("x", 8<<20)))}
			got, err := parseMessageMetadata(r)
			if err != nil {
				t.Fatal(err)
			}
			if (len(got.Attachments) > 0) != tc.attached {
				t.Fatal("attachment indicator lost")
			}
			t.Logf("metadata read %d bytes", r.read)
			if r.read > 64<<10 {
				t.Fatalf("metadata read %d bytes of payload", r.read)
			}
		})
	}
}
