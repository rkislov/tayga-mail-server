package flowsync

import (
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"testing"
	"time"
)

func BenchmarkEWSMailListing1000(b *testing.B) {
	ctx, st, u, eas, _, _ := syncFixture(b)
	mb, err := st.EnsureMailbox(ctx, u.ID, "INBOX", "")
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		_, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, Subject: "Indexed", MessageID: fmt.Sprintf("<bench-%d@example.com>", i), DateHdr: "Mon, 19 Oct 2026 12:34:56 +0300", InternalDate: time.Now()})
		if err != nil {
			b.Fatal(err)
		}
	}
	h := &ewsHandler{store: st, ms: eas.ms}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := h.findMailItems(ctx, u, mb.ID); err != nil {
			b.Fatal(err)
		}
	}
}
