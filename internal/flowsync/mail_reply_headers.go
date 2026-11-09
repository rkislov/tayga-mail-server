package flowsync

import (
	"bytes"
	"fmt"
	"net/mail"
	"strings"
)

// ReplaceMime means the body is complete, not that the device supplied all
// threading headers. Add only missing RFC links, preserving every MIME body
// byte (including signed content and large attachments) and existing links.
func ensureReplyHeaders(raw []byte, parent msgHeaders) ([]byte, error) {
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	ids := conversationIDs(parent.MessageID)
	if len(ids) != 1 {
		return raw, nil
	}
	var added strings.Builder
	if message.Header.Get("In-Reply-To") == "" {
		fmt.Fprintf(&added, "In-Reply-To: %s\r\n", ids[0])
	}
	if message.Header.Get("References") == "" {
		refs := conversationIDs(parent.References)
		if len(refs) == 0 {
			refs = conversationIDs(parent.InReplyTo)
		}
		// Retain the root and immediate parent for long conversations.
		if len(refs) > 62 {
			refs = append(refs[:1], refs[len(refs)-61:]...)
		}
		refs = append(refs, ids[0])
		fmt.Fprintf(&added, "References: %s\r\n", strings.Join(refs, " "))
	}
	if added.Len() == 0 {
		return raw, nil
	}
	return append([]byte(added.String()), raw...), nil
}
