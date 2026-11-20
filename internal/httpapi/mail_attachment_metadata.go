package httpapi

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tayga/tms/internal/mailsearch"
)

type mailListMetadata struct {
	HasAttachments bool
	Snippet        string
}

// Messages are immutable in the mail store. Cache only small list metadata, never
// decoded attachments, and bound memory independently of mailbox size.
var mailMetadataCache = struct {
	sync.Mutex
	entries map[string]mailListMetadata
	keys    []string
}{entries: make(map[string]mailListMetadata)}

func mailPreview(message parsedMsg) string {
	text := message.Text
	if strings.TrimSpace(text) == "" {
		text = mailsearch.ExtractHTMLText(message.HTML)
	}
	preview := []rune(strings.Join(strings.Fields(text), " "))
	if len(preview) > 180 {
		return string(preview[:180]) + "…"
	}
	return string(preview)
}

func (s *Server) mailListMetadata(filePath string) mailListMetadata {
	key := fmt.Sprintf("%p:%s", s.ms, filePath)
	mailMetadataCache.Lock()
	value, ok := mailMetadataCache.entries[key]
	mailMetadataCache.Unlock()
	if ok {
		return value
	}
	raw, err := s.ms.Read(filePath)
	if err != nil {
		return mailListMetadata{}
	}
	message := parseMIMEMessage(raw)
	value.Snippet = mailPreview(message)
	for _, attachment := range message.Attachments {
		if attachment.ContentID == "" {
			value.HasAttachments = true
			break
		}
	}
	mailMetadataCache.Lock()
	defer mailMetadataCache.Unlock()
	if _, exists := mailMetadataCache.entries[key]; exists {
		return mailMetadataCache.entries[key]
	}
	if len(mailMetadataCache.keys) >= 2048 {
		delete(mailMetadataCache.entries, mailMetadataCache.keys[0])
		mailMetadataCache.keys = mailMetadataCache.keys[1:]
	}
	mailMetadataCache.entries[key] = value
	mailMetadataCache.keys = append(mailMetadataCache.keys, key)
	return value
}

func (s *Server) hasMailAttachments(filePath string) bool {
	return s.mailListMetadata(filePath).HasAttachments
}
