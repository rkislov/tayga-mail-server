package imapserver

import (
	"bufio"
	"bytes"
	"io"
	"mime"
	"net/mail"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend/backendutil"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/textproto"
	"github.com/tayga/tms/internal/mailcharset"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func fetchMessage(ms *mailstore.Store, msg *storage.Message, seqNum uint32, items []imap.FetchItem) (*imap.Message, error) {
	body, err := ms.Read(msg.FilePath)
	if err != nil {
		return nil, err
	}
	flags := storage.ParseFlags(msg.Flags)
	fetched := imap.NewMessage(seqNum, items)
	for _, item := range items {
		switch item {
		case imap.FetchEnvelope:
			hdr, _, _ := headerAndBody(body)
			fetched.Envelope = fetchEnvelope(hdr)
		case imap.FetchBody, imap.FetchBodyStructure:
			hdr, r, _ := headerAndBody(body)
			fetched.BodyStructure, _ = backendutil.FetchBodyStructure(hdr, r, item == imap.FetchBodyStructure)
		case imap.FetchFlags:
			fetched.Flags = flags
		case imap.FetchInternalDate:
			fetched.InternalDate = msg.InternalDate
		case imap.FetchRFC822Size:
			fetched.Size = uint32(msg.Size)
		case imap.FetchUid:
			fetched.Uid = uint32(msg.UID)
		default:
			section, err := imap.ParseBodySectionName(item)
			if err != nil {
				break
			}
			hdr, r, err := headerAndBody(body)
			if err != nil {
				return nil, err
			}
			l, err := backendutil.FetchBodySection(hdr, r, section)
			if err != nil {
				return nil, err
			}
			fetched.Body[section] = l
		}
	}
	return fetched, nil
}

// Parse addresses with a charset-aware word decoder, before interpreting names
// as address syntax. Decoding the entire header first could turn an encoded
// comma or angle bracket inside a display name into an extra address.
func fetchEnvelope(h textproto.Header) *imap.Envelope {
	parser := mail.AddressParser{WordDecoder: &mime.WordDecoder{CharsetReader: mailcharset.Reader}}
	addresses := func(field string) []*imap.Address {
		parsed, err := parser.ParseList(h.Get(field))
		if err != nil {
			return nil
		}
		var result []*imap.Address
		for _, addr := range parsed {
			local, host, _ := strings.Cut(addr.Address, "@")
			result = append(result, &imap.Address{PersonalName: addr.Name, MailboxName: local, HostName: host})
		}
		return result
	}
	env := &imap.Envelope{
		Subject: h.Get("Subject"), From: addresses("From"), Sender: addresses("Sender"),
		ReplyTo: addresses("Reply-To"), To: addresses("To"), Cc: addresses("Cc"), Bcc: addresses("Bcc"),
		InReplyTo: h.Get("In-Reply-To"), MessageId: h.Get("Message-ID"),
	}
	env.Date, _ = mail.ParseDate(h.Get("Date"))
	if len(env.Sender) == 0 {
		env.Sender = env.From
	}
	if len(env.ReplyTo) == 0 {
		env.ReplyTo = env.From
	}
	return env
}

func matchMessage(ms *mailstore.Store, msg *storage.Message, seqNum uint32, criteria *imap.SearchCriteria) (bool, error) {
	body, err := ms.Read(msg.FilePath)
	if err != nil {
		return false, err
	}
	e, err := message.Read(bytes.NewReader(body))
	if err != nil && !message.IsUnknownCharset(err) && !message.IsUnknownEncoding(err) {
		// Fall back to flag/uid-only matching via empty entity is hard; treat as no match on parse fail for body criteria
		e = nil
	}
	flags := storage.ParseFlags(msg.Flags)
	if e == nil {
		// Minimal match without entity: only UID/seq/flags/date-ish via backendutil needs entity.
		// Use a tiny synthetic message for header-less match.
		e, _ = message.Read(bytes.NewReader([]byte("Subject: \r\n\r\n")))
	}
	return backendutil.Match(e, seqNum, uint32(msg.UID), msg.InternalDate, flags, criteria)
}

func headerAndBody(raw []byte) (textproto.Header, io.Reader, error) {
	body := bufio.NewReader(bytes.NewReader(raw))
	hdr, err := textproto.ReadHeader(body)
	return hdr, body, err
}
