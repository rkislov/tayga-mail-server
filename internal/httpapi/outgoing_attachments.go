package httpapi

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"
)

type outgoingAttachment struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func attachOutgoingFiles(raw []byte, files []outgoingAttachment) ([]byte, error) {
	if len(files) == 0 {
		return raw, nil
	}
	if len(files) > 20 {
		return nil, fmt.Errorf("too many attachments")
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	split := bytes.Index(raw, []byte("Content-Type:"))
	if split < 0 {
		return nil, fmt.Errorf("invalid MIME message")
	}
	part := raw[split:]
	sep := bytes.Index(part, []byte("\r\n\r\n"))
	if sep < 0 {
		return nil, fmt.Errorf("invalid MIME body")
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", strings.TrimSpace(string(part[len("Content-Type:"):sep])))
	writer, err := mw.CreatePart(header)
	if err != nil {
		return nil, err
	}
	writer.Write(part[sep+4:])
	total := 0
	for _, file := range files {
		data, err := base64.StdEncoding.DecodeString(file.Data)
		if err != nil {
			return nil, fmt.Errorf("invalid attachment data")
		}
		total += len(data)
		if total > 10<<20 {
			return nil, fmt.Errorf("attachments exceed 10 MiB; upload through Files")
		}
		name := strings.ReplaceAll(strings.ReplaceAll(file.Name, "\r", ""), "\n", "")
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", "application/octet-stream")
		h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
		h.Set("Content-Transfer-Encoding", "base64")
		p, err := mw.CreatePart(h)
		if err != nil {
			return nil, err
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		for len(encoded) > 76 {
			fmt.Fprintf(p, "%s\r\n", encoded[:76])
			encoded = encoded[76:]
		}
		fmt.Fprintf(p, "%s\r\n", encoded)
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	out := append([]byte{}, raw[:split]...)
	out = append(out, []byte("Content-Type: multipart/mixed; boundary="+mw.Boundary()+"\r\n\r\n")...)
	return append(out, body.Bytes()...), nil
}
