package fetchmail

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/tayga/tms/internal/storage"
)

func (s *Service) imap(ctx context.Context, a *Source, conn net.Conn, password string, test bool) (int64, bool, error) {
	var err error
	if a.Security == "starttls" {
		conn, err = imapStartTLS(ctx, conn, s.tlsConfig(a))
		if err != nil {
			return 0, false, fmt.Errorf("IMAP STARTTLS: %w", err)
		}
	}
	c, err := client.New(limitIMAP(conn, s.maxSize))
	if err != nil {
		return 0, false, fmt.Errorf("IMAP greeting: %w", err)
	}
	defer c.Terminate()
	c.Timeout = time.Minute
	if err = c.Login(a.Username, password); err != nil {
		return 0, false, errors.New("IMAP authentication failed; check username and application password")
	}
	box, err := c.Select(a.SourceFolder, true)
	if err != nil {
		return 0, false, errors.New("IMAP source folder is unavailable")
	}
	if test {
		return 0, false, nil
	}
	criteria := imap.NewSearchCriteria()
	uids, err := c.UidSearch(criteria)
	if err != nil {
		return 0, false, fmt.Errorf("IMAP UID search: %w", err)
	}
	if len(uids) > 100000 {
		return 0, false, errors.New("source folder exceeds 100000 messages; select a smaller source folder")
	}
	sort.Slice(uids, func(i, j int) bool { return uids[i] < uids[j] })
	prefix := fmt.Sprintf("%s:%d:", s.remotePrefix(a), box.UidValidity)
	var copied int64
	var firstErr error
	attempted := 0
	for _, uid := range uids {
		if err = ctx.Err(); err != nil {
			return copied, false, err
		}
		key := prefix + strconv.FormatUint(uint64(uid), 10)
		seen, e := s.seen(ctx, a, key)
		if e != nil {
			return copied, false, e
		}
		if seen {
			continue
		}
		if attempted == 100 {
			return copied, true, firstErr
		}
		set := new(imap.SeqSet)
		set.AddNum(uid)
		metadata, e := fetchOne(c, set, []imap.FetchItem{imap.FetchUid, imap.FetchRFC822Size, imap.FetchFlags, imap.FetchInternalDate})
		if e != nil {
			return copied, false, e
		}
		if metadata != nil && metadata.Uid != uid {
			return copied, false, errors.New("IMAP returned an unexpected message UID")
		}
		if metadata == nil {
			continue
		}
		if int64(metadata.Size) > s.maxSize {
			firstErr = errors.New("some messages exceed the server message size limit")
			continue
		}
		attempted++
		section := &imap.BodySectionName{Peek: true}
		message, e := fetchOne(c, set, []imap.FetchItem{imap.FetchUid, section.FetchItem()})
		if e != nil {
			return copied, false, e
		}
		if message != nil && message.Uid != uid {
			return copied, false, errors.New("IMAP returned an unexpected message UID")
		}
		if message == nil {
			continue
		}
		body := message.GetBody(section)
		if body == nil {
			return copied, false, errors.New("IMAP returned no message body")
		}
		raw, e := io.ReadAll(io.LimitReader(body, s.maxSize+1))
		if e != nil {
			return copied, false, e
		}
		if int64(len(raw)) > s.maxSize {
			return copied, false, errors.New("IMAP message exceeds size limit")
		}
		flags := []string{}
		for _, flag := range metadata.Flags {
			switch flag {
			case imap.SeenFlag, imap.AnsweredFlag, imap.FlaggedFlag, imap.DraftFlag:
				flags = append(flags, flag)
			}
		}
		added, e := s.deliver(ctx, a, key, raw, storage.NormalizeFlags(flags), metadata.InternalDate)
		if e != nil {
			return copied, false, e
		}
		if added {
			copied++
		}
	}
	return copied, false, firstErr
}
func fetchOne(c *client.Client, set *imap.SeqSet, items []imap.FetchItem) (*imap.Message, error) {
	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() { done <- c.UidFetch(set, items, messages) }()
	var found *imap.Message
	for message := range messages {
		found = message
	}
	return found, <-done
}

// POP3 deliberately uses UIDL and RETR only: no DELE, so originals remain intact.
func (s *Service) pop3(ctx context.Context, a *Source, conn net.Conn, password string, test bool) (int64, bool, error) {
	reader := bufio.NewReader(conn)
	read := func() (string, error) { return popLine(reader) }
	command := func(line string) (string, error) {
		if _, e := io.WriteString(conn, line+"\r\n"); e != nil {
			return "", e
		}
		reply, e := read()
		if e != nil {
			return "", e
		}
		if !strings.HasPrefix(reply, "+OK") {
			return "", errors.New("POP3 command rejected")
		}
		return reply, nil
	}
	greeting, err := read()
	if err != nil || !strings.HasPrefix(greeting, "+OK") {
		return 0, false, errors.New("POP3 greeting rejected")
	}
	if a.Security == "starttls" {
		if _, err = command("STLS"); err != nil {
			return 0, false, errors.New("POP3 STARTTLS unavailable")
		}
		tc := tls.Client(conn, s.tlsConfig(a))
		if err = tc.HandshakeContext(ctx); err != nil {
			return 0, false, fmt.Errorf("POP3 TLS verification: %w", err)
		}
		conn = tc
		reader = bufio.NewReader(tc)
	}
	if _, err = command("USER " + a.Username); err != nil {
		return 0, false, errors.New("POP3 username rejected")
	}
	if _, err = command("PASS " + password); err != nil {
		return 0, false, errors.New("POP3 authentication failed; check application password")
	}
	if _, err = command("UIDL"); err != nil {
		return 0, false, errors.New("POP3 server must support UIDL to avoid duplicate mail")
	}
	type entry struct {
		number int
		uid    string
	}
	entries := []entry{}
	total := 0
	seenUID := map[string]bool{}
	for {
		line, e := read()
		if e != nil {
			return 0, false, e
		}
		if line == "." {
			break
		}
		total += len(line)
		if total > 4<<20 || len(entries) >= 100000 {
			return 0, false, errors.New("POP3 UID list exceeds limit")
		}
		f := strings.Fields(line)
		if len(f) != 2 || len(f[1]) > 70 {
			return 0, false, errors.New("invalid POP3 UIDL response")
		}
		number, e := strconv.Atoi(f[0])
		if e != nil || number < 1 || seenUID[f[1]] {
			return 0, false, errors.New("invalid POP3 message identity")
		}
		seenUID[f[1]] = true
		entries = append(entries, entry{number, f[1]})
	}
	if test {
		command("QUIT")
		return 0, false, nil
	}
	var copied int64
	var firstErr error
	attempted := 0
	prefix := s.remotePrefix(a) + ":"
	for _, item := range entries {
		if err = ctx.Err(); err != nil {
			return copied, false, err
		}
		key := prefix + item.uid
		seen, e := s.seen(ctx, a, key)
		if e != nil {
			return copied, false, e
		}
		if seen {
			continue
		}
		if attempted == 100 {
			return copied, true, firstErr
		}
		line, e := command("LIST " + strconv.Itoa(item.number))
		if e != nil {
			return copied, false, e
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return copied, false, errors.New("invalid POP3 LIST response")
		}
		size, e := strconv.ParseInt(fields[2], 10, 64)
		if e != nil || size < 0 {
			return copied, false, errors.New("invalid POP3 message size")
		}
		if size > s.maxSize {
			firstErr = errors.New("some messages exceed the server message size limit")
			continue
		}
		attempted++
		if _, e = command("RETR " + strconv.Itoa(item.number)); e != nil {
			return copied, false, e
		}
		raw, e := io.ReadAll(io.LimitReader(textproto.NewReader(reader).DotReader(), s.maxSize+1))
		if e != nil {
			return copied, false, e
		}
		if int64(len(raw)) > s.maxSize {
			return copied, false, errors.New("POP3 message exceeds size limit")
		}
		// DotReader canonicalizes CRLF to LF. Restore RFC822 line endings for storage.
		raw = []byte(strings.ReplaceAll(string(raw), "\n", "\r\n"))
		if int64(len(raw)) > s.maxSize {
			return copied, false, errors.New("POP3 message exceeds size limit")
		}
		added, e := s.deliver(ctx, a, key, raw, "", time.Now().UTC())
		if e != nil {
			return copied, false, e
		}
		if added {
			copied++
		}
	}
	command("QUIT")
	return copied, false, firstErr
}
func popLine(r *bufio.Reader) (string, error) {
	var b []byte
	for {
		p, err := r.ReadSlice('\n')
		if len(b)+len(p) > 16384 {
			return "", errors.New("POP3 response line exceeds limit")
		}
		b = append(b, p...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(b), "\r\n"), nil
	}
}
