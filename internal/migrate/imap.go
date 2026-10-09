package migrate

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
)

type imapOptions struct {
	SkipTLSVerify bool     `json:"skip_tls_verify"`
	Folders       []string `json:"folders"` // empty = all
}

func (s *Service) runIMAP(ctx context.Context, job *Job) error {
	password, err := decryptPassword(s.key, job.PasswordCiphertext)
	if err != nil {
		return err
	}
	user, err := s.store.GetUserByID(ctx, job.UserID)
	if err != nil {
		return err
	}
	if _, err := s.ms.EnsureUser(user.Email); err != nil {
		return err
	}

	var opts imapOptions
	_ = json.Unmarshal([]byte(job.Options), &opts)
	addr := fmt.Sprintf("%s:%d", job.Host, job.Port)
	var c *client.Client
	if job.TLS {
		c, err = client.DialTLS(addr, &tls.Config{ServerName: job.Host, MinVersion: tls.VersionTLS12, InsecureSkipVerify: opts.SkipTLSVerify})
	} else {
		c, err = client.Dial(addr)
	}
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = c.Logout() }()

	if err := c.Login(job.Username, password); err != nil {
		return fmt.Errorf("login: %w", err)
	}

	mailboxes := make(chan *imap.MailboxInfo, 32)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()
	var boxes []string
	for m := range mailboxes {
		if m == nil || m.Name == "" {
			continue
		}
		boxes = append(boxes, m.Name)
	}
	if err := <-done; err != nil {
		return fmt.Errorf("list: %w", err)
	}
	if len(opts.Folders) > 0 {
		want := map[string]bool{}
		for _, f := range opts.Folders {
			want[strings.ToLower(f)] = true
		}
		var filtered []string
		for _, b := range boxes {
			if want[strings.ToLower(b)] {
				filtered = append(filtered, b)
			}
		}
		boxes = filtered
	}

	copied, skipped, errs := job.Copied, job.Skipped, job.Errors
	for _, box := range boxes {
		if ctx.Err() != nil || s.isCancelled(ctx, job.ID) {
			job.Copied, job.Skipped, job.Errors = copied, skipped, errs
			return ctx.Err()
		}
		localName := mapIMAPFolder(box)
		root, err := s.ms.EnsureFolder(user.Email, localName)
		if err != nil {
			errs++
			_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, err.Error(), `{"folder":`+jsonString(box)+`}`)
			continue
		}
		mb, err := s.store.EnsureMailbox(ctx, user.ID, localName, root)
		if err != nil {
			errs++
			continue
		}
		// Archive sizes in the index are compressed on disk; deduplication uses
		// the uncompressed RFC822 size, as supplied by the source IMAP server.
		archiveSizes := map[string]map[int64]bool{}
		if strings.EqualFold(localName, "Archive") {
			existing, e := s.store.ListMessages(ctx, mb.ID)
			if e != nil {
				return fmt.Errorf("archive index: %w", e)
			}
			for _, m := range existing {
				raw, e := s.ms.Read(m.FilePath)
				if e != nil {
					return fmt.Errorf("archive read: %w", e)
				}
				messageID := m.MessageID
				if messageID == "" {
					messageID = mailsearch.ParseDocument(raw).MessageID
				}
				if messageID == "" {
					continue
				}
				if archiveSizes[messageID] == nil {
					archiveSizes[messageID] = map[int64]bool{}
				}
				archiveSizes[messageID][int64(len(raw))] = true
			}
		}
		if _, err := c.Select(box, true); err != nil {
			errs++
			_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, err.Error(), `{"folder":`+jsonString(box)+`}`)
			continue
		}
		if c.Mailbox() == nil || c.Mailbox().Messages == 0 {
			continue
		}
		seqset := new(imap.SeqSet)
		seqset.AddRange(1, c.Mailbox().Messages)
		section := &imap.BodySectionName{}
		items := []imap.FetchItem{section.FetchItem(), imap.FetchFlags, imap.FetchInternalDate, imap.FetchUid, imap.FetchRFC822Size}
		messages := make(chan *imap.Message, 16)
		fetchDone := make(chan error, 1)
		go func() {
			fetchDone <- c.Fetch(seqset, items, messages)
		}()
		for msg := range messages {
			if ctx.Err() != nil || s.isCancelled(ctx, job.ID) {
				job.Copied, job.Skipped, job.Errors = copied, skipped, errs
				return ctx.Err()
			}
			if msg == nil {
				continue
			}
			r := msg.GetBody(section)
			if r == nil {
				errs++
				continue
			}
			raw, err := io.ReadAll(r)
			if err != nil {
				errs++
				continue
			}
			tmp := &storage.Message{}
			mailsearch.ApplyHeaders(tmp, raw)
			exists, err := s.store.MessageExistsByMessageID(ctx, mb.ID, tmp.MessageID, int64(len(raw)))
			if err != nil {
				return fmt.Errorf("duplicate lookup: %w", err)
			}
			if exists || archiveSizes[tmp.MessageID][int64(len(raw))] {
				skipped++
				continue
			}
			if qerr := storage.EnsureQuota(ctx, s.store, user, int64(len(raw))); qerr != nil {
				errs++
				_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, qerr.Error(), `{"folder":`+jsonString(box)+`}`)
				continue
			}
			rel, size, err := s.ms.Deliver(user.Email, localName, raw)
			if err != nil {
				errs++
				continue
			}
			flags := imapFlagsToStorage(msg.Flags)
			date := msg.InternalDate
			if date.IsZero() {
				date = time.Now().UTC()
			}
			ins := &storage.Message{
				MailboxID:    mb.ID,
				Size:         size,
				Flags:        flags,
				InternalDate: date,
				FilePath:     rel,
				MessageID:    tmp.MessageID,
				Subject:      tmp.Subject,
				FromAddr:     tmp.FromAddr,
				ToAddr:       tmp.ToAddr,
				DateHdr:      tmp.DateHdr,
				Archived:     strings.EqualFold(localName, "Archive"),
			}
			inserted, err := s.store.InsertMessage(ctx, ins)
			if err != nil {
				errs++
				_ = s.ms.Delete(rel)
				job.LastError = "index imported message: " + err.Error()
				_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, job.LastError, `{"folder":`+jsonString(box)+`}`)
				continue
			}
			if strings.EqualFold(localName, "Archive") && tmp.MessageID != "" {
				if archiveSizes[tmp.MessageID] == nil {
					archiveSizes[tmp.MessageID] = map[int64]bool{}
				}
				archiveSizes[tmp.MessageID][int64(len(raw))] = true
			}
			_ = mailsearch.Index(ctx, s.store, inserted.ID, raw)
			copied++
			if (copied+skipped+errs)%25 == 0 {
				_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, "", `{"folder":`+jsonString(box)+`}`)
			}
		}
		if err := <-fetchDone; err != nil {
			errs++
			_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, err.Error(), `{"folder":`+jsonString(box)+`}`)
		}
		_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, "", `{"folder":`+jsonString(box)+`}`)
	}
	job.Copied, job.Skipped, job.Errors = copied, skipped, errs
	_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, "", "{}")
	return nil
}

func mapIMAPFolder(name string) string {
	n := strings.Trim(name, "/")
	lower := strings.ToLower(n)
	switch lower {
	case "inbox", "inbox.":
		return "INBOX"
	case "sent", "sent items", "sent messages", "[gmail]/sent mail":
		return "Sent"
	case "drafts", "[gmail]/drafts":
		return "Drafts"
	case "trash", "deleted items", "[gmail]/trash":
		return "Trash"
	case "junk", "spam", "junk e-mail", "[gmail]/spam":
		return "Junk"
	case "archive", "[gmail]/all mail":
		return "Archive"
	}
	// flatten hierarchy separators commonly used by IMAP
	n = strings.ReplaceAll(n, ".", "/")
	n = strings.TrimPrefix(n, "INBOX/")
	if n == "" {
		return "INBOX"
	}
	return n
}

func imapFlagsToStorage(flags []string) string {
	var out []string
	for _, f := range flags {
		switch strings.ToLower(f) {
		case `\seen`:
			out = append(out, `\Seen`)
		case `\flagged`:
			out = append(out, `\Flagged`)
		case `\answered`:
			out = append(out, `\Answered`)
		case `\deleted`:
			out = append(out, `\Deleted`)
		case `\draft`:
			out = append(out, `\Draft`)
		}
	}
	return storage.NormalizeFlags(out)
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
