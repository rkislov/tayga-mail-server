package mailsearch

import (
	"context"

	"github.com/tayga/tms/internal/storage"
)

// Indexer can upsert FTS rows (satisfied by storage.Driver / *storage.Store).
type Indexer interface {
	UpsertMessageSearch(ctx context.Context, messageID, subject, fromAddr, toAddr, body string) error
}

// ApplyHeaders copies denormalized header fields from raw RFC822 onto msg.
func ApplyHeaders(msg *storage.Message, raw []byte) {
	if msg == nil || len(raw) == 0 {
		return
	}
	doc := ParseDocument(raw)
	if doc.Subject != "" {
		msg.Subject = doc.Subject
	}
	msg.FromAddr = doc.From
	msg.ToAddr = doc.To
	msg.DateHdr = doc.Date
}

// Index upserts FTS for messageID from raw RFC822 bytes.
func Index(ctx context.Context, st Indexer, messageID string, raw []byte) error {
	if st == nil || messageID == "" {
		return nil
	}
	doc := ParseDocument(raw)
	return st.UpsertMessageSearch(ctx, messageID, doc.Subject, doc.From, doc.To, doc.Body)
}

// IndexDocument upserts FTS from an already-parsed document.
func IndexDocument(ctx context.Context, st Indexer, messageID string, doc Document) error {
	if st == nil || messageID == "" {
		return nil
	}
	return st.UpsertMessageSearch(ctx, messageID, doc.Subject, doc.From, doc.To, doc.Body)
}
