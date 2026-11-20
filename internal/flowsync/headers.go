// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"bufio"
	"bytes"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/mailstore"
)

type msgHeaders struct {
	MessageID, References, InReplyTo string
	Subject                          string
	From                             string
	SentDate                         time.Time
}

func readMsgHeaders(ms *mailstore.Store, filePath string) msgHeaders {
	h := msgHeaders{Subject: "(no subject)"}
	if ms == nil || filePath == "" {
		return h
	}
	data, err := os.ReadFile(ms.Abs(filePath))
	if err != nil {
		return h
	}
	// headers only — stop at blank line
	sc := bufio.NewScanner(bytes.NewReader(data))
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
		case "message-id":
			h.MessageID = v
		case "references":
			h.References = v
		case "in-reply-to":
			h.InReplyTo = v
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
