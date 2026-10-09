package httpapi

import (
	"fmt"
	"sync"
)

var mailAttachmentPresence sync.Map

func (s *Server) hasMailAttachments(filePath string) bool {
	key := fmt.Sprintf("%p:%s", s.ms, filePath)
	if value, ok := mailAttachmentPresence.Load(key); ok {
		return value.(bool)
	}
	raw, err := s.ms.Read(filePath)
	if err != nil {
		return false
	}
	message := parseMIMEMessage(raw)
	has := false
	for _, attachment := range message.Attachments {
		if attachment.ContentID == "" {
			has = true
			break
		}
	}
	mailAttachmentPresence.Store(key, has)
	return has
}
