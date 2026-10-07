package flowsync

import (
	"bufio"
	"bytes"
	"os"
	"strings"

	"github.com/tayga/tms/internal/mailstore"
)

type msgHeaders struct {
	Subject string
	From    string
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
				h.Subject = v
			}
		case "from":
			h.From = v
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
