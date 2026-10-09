package httpapi

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/mail"
	"net/textproto"
	"path"
	"strconv"
	"strings"

	"github.com/tayga/tms/internal/mailsearch"
	"golang.org/x/net/html/charset"
)

type mailAttachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	ContentID   string `json:"content_id,omitempty"`
	Data        []byte `json:"-"`
}

func parseMIMEMessage(raw []byte) parsedMsg {
	out := parsedMsg{Subject: "(no subject)"}
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		out.Text = string(raw)
		return out
	}
	if v := m.Header.Get("Subject"); v != "" {
		out.Subject = mailsearch.DecodeHeader(v)
	}
	out.From = mailsearch.DecodeHeader(m.Header.Get("From"))
	out.ReplyTo = mailsearch.DecodeHeader(m.Header.Get("Reply-To"))
	out.To = mailsearch.DecodeHeader(m.Header.Get("To"))
	out.Cc = mailsearch.DecodeHeader(m.Header.Get("Cc"))
	out.Date = m.Header.Get("Date")
	var walk func(textproto.MIMEHeader, io.Reader, int)
	walk = func(h textproto.MIMEHeader, r io.Reader, depth int) {
		if depth > 20 {
			return
		}
		switch strings.ToLower(strings.TrimSpace(h.Get("Content-Transfer-Encoding"))) {
		case "base64":
			r = base64.NewDecoder(base64.StdEncoding, r)
		case "quoted-printable":
			r = quotedprintable.NewReader(r)
		}
		media, params, _ := mime.ParseMediaType(h.Get("Content-Type"))
		if media == "" {
			media = "text/plain"
		}
		if strings.HasPrefix(media, "multipart/") {
			if params["boundary"] == "" {
				return
			}
			mr := multipart.NewReader(r, params["boundary"])
			for {
				p, err := mr.NextRawPart()
				if err != nil {
					break
				}
				walk(p.Header, p, depth+1)
				p.Close()
			}
			return
		}
		disposition, dp, _ := mime.ParseMediaType(h.Get("Content-Disposition"))
		filename := dp["filename"]
		if filename == "" {
			filename = params["name"]
		}
		cid := strings.Trim(h.Get("Content-ID"), " <>")
		if disposition == "attachment" || filename != "" || cid != "" && strings.HasPrefix(media, "image/") {
			data, err := io.ReadAll(io.LimitReader(r, 64<<20))
			if err != nil {
				return
			}
			filename = path.Base(strings.ReplaceAll(mailsearch.DecodeHeader(filename), "\\", "/"))
			if filename == "." || filename == "" {
				filename = "attachment"
			}
			out.Attachments = append(out.Attachments, mailAttachment{ID: strconv.Itoa(len(out.Attachments)), Filename: filename, ContentType: media, Size: len(data), ContentID: cid, Data: data})
			return
		}
		if media != "text/plain" && media != "text/html" {
			return
		}
		if label := params["charset"]; label != "" {
			if converted, err := charset.NewReaderLabel(label, r); err == nil {
				r = converted
			}
		}
		data, err := io.ReadAll(io.LimitReader(r, 4<<20))
		if err != nil {
			return
		}
		if media == "text/html" && out.HTML == "" {
			out.HTML = string(data)
		}
		if media == "text/plain" && out.Text == "" {
			out.Text = string(data)
		}
	}
	walk(textproto.MIMEHeader(m.Header), m.Body, 0)
	// Embed only passive raster images; never executable SVG or attachment HTML.
	for _, a := range out.Attachments {
		if a.ContentID != "" && (a.ContentType == "image/png" || a.ContentType == "image/jpeg" || a.ContentType == "image/gif" || a.ContentType == "image/webp") {
			out.HTML = strings.ReplaceAll(out.HTML, "cid:"+a.ContentID, "data:"+a.ContentType+";base64,"+base64.StdEncoding.EncodeToString(a.Data))
		}
	}
	return out
}

func (s *Server) mailDownloadAttachment(w http.ResponseWriter, r *http.Request, au *authUser, messageID, attachmentID string) {
	msg, _, code := s.messageOwned(r, au, messageID)
	if code != 0 {
		http.Error(w, http.StatusText(code), code)
		return
	}
	raw, err := s.ms.Read(msg.FilePath)
	if err != nil {
		http.Error(w, "read failed", 500)
		return
	}
	parsed := parseMIMEMessage(raw)
	for _, a := range parsed.Attachments {
		if a.ID == attachmentID {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": a.Filename}))
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "private, no-store")
			w.Write(a.Data)
			return
		}
	}
	http.NotFound(w, r)
}
