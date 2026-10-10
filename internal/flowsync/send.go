// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"context"
	"fmt"
	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
	"io"
	"net/mail"
	"sort"
	"strings"
	"sync"
	"time"
)

type SubmitFunc func(context.Context, *storage.User, []string, []byte) error
type mailSubmission struct {
	store  storage.Driver
	ms     *mailstore.Store
	submit SubmitFunc
	mu     sync.Mutex
}

func (s *mailSubmission) send(ctx context.Context, u *storage.User, clientID string, raw []byte, save bool) error {
	if s == nil || s.submit == nil {
		return fmt.Errorf("SMTP submission unavailable")
	}
	if len(raw) == 0 || len(raw) > 32<<20 {
		return fmt.Errorf("invalid message size")
	}
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	from, err := message.Header.AddressList("From")
	if err != nil || len(from) != 1 || !strings.EqualFold(from[0].Address, u.Email) {
		return fmt.Errorf("sender must match authenticated account")
	}
	recipients := []string{}
	seen := map[string]bool{}
	for _, field := range []string{"To", "Cc", "Bcc"} {
		if message.Header.Get(field) == "" {
			continue
		}
		addresses, err := message.Header.AddressList(field)
		if err != nil {
			return err
		}
		for _, a := range addresses {
			key := strings.ToLower(a.Address)
			if !seen[key] {
				recipients = append(recipients, a.Address)
				seen[key] = true
			}
		}
	}
	if len(recipients) == 0 || len(recipients) > 100 {
		return fmt.Errorf("invalid recipient count")
	}
	fingerprint := digest(string(raw))
	if clientID == "" {
		clientID = storage.NewID()
	}
	receipt := "send:" + digest(clientID)
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.store.GetFlowSyncState(ctx, u.ID, "submission", receipt)
	if err != nil {
		return err
	}
	if previous != "" {
		if previous != fingerprint {
			return fmt.Errorf("client ID reused with different message")
		}
		return nil
	}
	data, err := io.ReadAll(message.Body)
	if err != nil {
		return err
	}
	header := message.Header
	delete(header, "Bcc")
	delete(header, "Content-Length")
	msgid := header.Get("Message-Id")
	if msgid == "" {
		msgid = "<" + digest(u.ID+clientID) + "@flowsync>"
		header["Message-Id"] = []string{msgid}
	}
	if header.Get("Date") == "" {
		header["Date"] = []string{time.Now().UTC().Format(time.RFC1123Z)}
	}
	keys := []string{}
	for key := range header {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var encoded bytes.Buffer
	for _, key := range keys {
		for _, value := range header[key] {
			fmt.Fprintf(&encoded, "%s: %s\r\n", key, strings.ReplaceAll(strings.ReplaceAll(value, "\r", ""), "\n", " "))
		}
	}
	encoded.WriteString("\r\n")
	encoded.Write(data)
	raw = encoded.Bytes()
	if err = s.submit(ctx, u, recipients, raw); err != nil {
		return err
	}
	// Record acceptance before saving the Sent copy, so a client retry cannot
	// resubmit a successfully accepted message after a local-copy failure.
	if err = s.store.PutFlowSyncState(ctx, u.ID, "submission", receipt, fingerprint); err != nil {
		return err
	}
	if save && s.ms != nil {
		path, err := s.ms.EnsureFolder(u.Email, "Sent")
		if err != nil {
			return err
		}
		mb, err := s.store.EnsureMailbox(ctx, u.ID, "Sent", path)
		if err != nil {
			return err
		}
		rel, size, err := s.ms.Deliver(u.Email, "Sent", raw)
		if err != nil {
			return err
		}
		record := &storage.Message{MailboxID: mb.ID, Size: size, Flags: `\Seen`, InternalDate: time.Now().UTC(), FilePath: rel, MessageID: msgid}
		mailsearch.ApplyHeaders(record, raw)
		inserted, err := s.store.InsertMessage(ctx, record)
		if err != nil {
			return err
		}
		_ = mailsearch.Index(ctx, s.store, inserted.ID, raw)
	}
	return nil
}
