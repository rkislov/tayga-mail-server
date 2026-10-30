// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"bufio"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

type msgHeaders struct {
	Subject  string
	From     string
	SentDate time.Time
}

func readMsgHeaders(ms *mailstore.Store, filePath string) msgHeaders {
	h := msgHeaders{Subject: "(no subject)"}
	if ms == nil || filePath == "" {
		return h
	}
	r, err := ms.Open(filePath)
	if err != nil {
		return h
	}
	defer r.Close()
	return parseMsgHeaders(r)
}

// SMTP, IMAP imports and submission already index these immutable MIME headers.
// Only legacy rows with missing metadata need a bounded file read.
func indexedMsgHeaders(ms *mailstore.Store, msg *storage.Message) msgHeaders {
	h := msgHeaders{Subject: msg.Subject, From: msg.FromAddr}
	h.SentDate, _ = mail.ParseDate(msg.DateHdr)
	if msg.Subject == "" || msg.DateHdr == "" {
		fallback := readMsgHeaders(ms, msg.FilePath)
		if msg.Subject == "" {
			h.Subject = fallback.Subject
		}
		if msg.DateHdr == "" {
			h.SentDate = fallback.SentDate
		}
	}
	return h
}

func parseMsgHeaders(r io.Reader) msgHeaders {
	h := msgHeaders{Subject: "(no subject)"}
	// headers only — stop at blank line
	sc := bufio.NewScanner(io.LimitReader(r, 1024*1024))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var cur string
	var val strings.Builder
	flush := func() {
		name := strings.ToLower(strings.TrimSpace(cur))
		v := strings.TrimSpace(val.String())
		switch name {
		case "subject":
			if v != "" {
				h.Subject = mailsearch.DecodeHeader(v)
			}
		case "from":
			h.From = mailsearch.DecodeHeader(v)
		case "date":
			h.SentDate, _ = mail.ParseDate(v)
		}
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			flush()
			break
		}
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			val.WriteByte(' ')
			val.WriteString(strings.TrimSpace(line))
			continue
		}
		flush()
		cur, val = "", strings.Builder{}
		if i := strings.IndexByte(line, ':'); i >= 0 {
			cur = line[:i]
			val.WriteString(strings.TrimSpace(line[i+1:]))
		}
	}
	flush()
	return h
}
