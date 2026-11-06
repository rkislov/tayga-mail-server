package flowsync

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var conversationReplyPrefix = regexp.MustCompile(`(?i)^(?:re|fw|fwd|aw|sv|ответ|пересл)(?:\[[0-9]+\])?:\s*`)
var conversationMessageID = regexp.MustCompile(`<[^<>\s]+>`)

func conversationIDs(value string) []string {
	ids := conversationMessageID.FindAllString(value, 65)
	if len(ids) > 64 {
		ids = append(ids[:1], ids[len(ids)-63:]...)
	}
	return ids
}
func conversationIDKey(value string) string { return strings.Trim(strings.TrimSpace(value), "<>") }
func conversationTopic(value string) string {
	value = strings.TrimSpace(value)
	for i := 0; i < 16; i++ {
		next := conversationReplyPrefix.ReplaceAllString(value, "")
		if next == value {
			break
		}
		value = next
	}
	return value
}
func validConversationIndex(value string) []byte {
	if len(value) > 512 {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(value), ""))
	if err != nil || len(data) < 22 || (len(data)-22)%5 != 0 || len(data) > 342 || data[0] != 1 {
		return nil
	}
	return data
}
func conversationFiletime(t time.Time) uint64 {
	if t.IsZero() || t.Year() < 1601 || t.Year() > 9999 {
		t = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return uint64(t.Unix()+11644473600)*10000000 + uint64(t.Nanosecond()/100)
}
func conversationChild(index []byte, id string, sent time.Time) []byte {
	var stamp [8]byte
	copy(stamp[1:6], index[1:6])
	root := binary.BigEndian.Uint64(stamp[:])
	current := conversationFiletime(sent)
	var delta uint64
	if current > root {
		delta = current - root
	}
	var block [5]byte
	// MS-OXOMSG 2.2.1.3: 31-bit delta plus the delta code and a random byte.
	if delta>>18 <= 0x7fffffff {
		binary.BigEndian.PutUint32(block[:4], uint32(delta>>18))
	} else {
		binary.BigEndian.PutUint32(block[:4], 0x80000000|uint32(delta>>23)&0x7fffffff)
	}
	hash := sha256.Sum256([]byte(id))
	block[4] = hash[0]
	return append(index, block[:]...)
}

func (h *ewsHandler) ewsConversationFields(ctx context.Context, u *storage.User, msg *storage.Message, content messageContent) (string, error) {
	own := content.MessageID
	if conversationIDKey(own) == "" {
		own = "<tayga:" + msg.ID + ">"
	}
	refs := conversationIDs(content.References)
	if len(refs) == 0 {
		refs = conversationIDs(content.InReplyTo)
	}
	// Recover the ancestor chain for mailers which supply only In-Reply-To.
	if len(refs) == 1 && content.References == "" {
		seen := map[string]bool{conversationIDKey(own): true}
		for depth := 0; depth < 16; depth++ {
			parentID := refs[0]
			key := conversationIDKey(parentID)
			if seen[key] {
				break
			}
			seen[key] = true
			parents, err := h.store.MessagesByInternetMessageIDs(ctx, u.ID, []string{parentID, key})
			if err != nil {
				return "", err
			}
			if len(parents) == 0 {
				break
			}
			hdr := readMsgHeaders(h.ms, parents[0].FilePath)
			ancestors := conversationIDs(hdr.References)
			if len(ancestors) > 0 {
				refs = append(ancestors, refs...)
				break
			}
			ancestors = conversationIDs(hdr.InReplyTo)
			if len(ancestors) == 0 {
				break
			}
			refs = append(ancestors, refs...)
		}
	}
	// Bound malformed cycles and repeated references, keeping the root first.
	chain := make([]string, 0, len(refs))
	seen := map[string]bool{conversationIDKey(own): true}
	for _, id := range refs {
		key := conversationIDKey(id)
		if !seen[key] {
			chain = append(chain, id)
			seen[key] = true
		}
	}
	if len(chain) > 64 {
		chain = append(chain[:1], chain[len(chain)-63:]...)
	}
	rootID := own
	if len(chain) > 0 {
		rootID = chain[0]
	}
	lookup := make([]string, 0, len(chain)*2)
	for _, id := range chain {
		lookup = append(lookup, id, conversationIDKey(id))
	}
	ancestors, err := h.store.MessagesByInternetMessageIDs(ctx, u.ID, lookup)
	if err != nil {
		return "", err
	}
	indexed := map[string]*storage.Message{}
	for _, m := range ancestors {
		key := conversationIDKey(m.MessageID)
		if indexed[key] == nil {
			indexed[key] = m
		}
	}
	rootDate := content.SentDate
	if rootDate.IsZero() {
		rootDate = msg.InternalDate
	}
	topic := content.ThreadTopic
	if topic == "" {
		topic = conversationTopic(content.Subject)
	}
	rootIndex := []byte(nil)
	if len(chain) > 0 {
		if root := indexed[conversationIDKey(rootID)]; root != nil {
			rootDate, _ = mail.ParseDate(root.DateHdr)
			if rootDate.IsZero() {
				rootDate = root.InternalDate
			}
			topic = conversationTopic(root.Subject)
			hdr := readMsgHeaders(h.ms, root.FilePath)
			if hdr.ThreadTopic != "" {
				topic = hdr.ThreadTopic
			}
			rootIndex = validConversationIndex(hdr.ThreadIndex)
		} else {
			rootDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		}
	}
	index := validConversationIndex(content.ThreadIndex)
	if index == nil {
		if rootIndex != nil {
			index = append([]byte(nil), rootIndex[:22]...)
		} else {
			index = make([]byte, 22)
			index[0] = 1
			var stamp [8]byte
			binary.BigEndian.PutUint64(stamp[:], conversationFiletime(rootDate))
			copy(index[1:6], stamp[1:6])
			guid := sha256.Sum256([]byte(u.ID + "\n" + conversationIDKey(rootID)))
			copy(index[6:22], guid[:16])
		}
		if len(chain) > 0 {
			for _, id := range append(append([]string(nil), chain[1:]...), own) {
				sent := content.SentDate
				if m := indexed[conversationIDKey(id)]; m != nil {
					sent, _ = mail.ParseDate(m.DateHdr)
					if sent.IsZero() {
						sent = m.InternalDate
					}
				}
				index = conversationChild(index, conversationIDKey(id), sent)
			}
		}
	}
	if topic == "" {
		topic = "(no subject)"
	}
	idHash := sha256.Sum256(append([]byte(u.ID+"\n"), index[6:22]...))
	return fmt.Sprintf(`<t:ConversationId Id="%s"/><t:ConversationIndex>%s</t:ConversationIndex><t:ConversationTopic>%s</t:ConversationTopic>`, base64.StdEncoding.EncodeToString(idHash[:16]), base64.StdEncoding.EncodeToString(index), xmlEscape(topic)), nil
}
