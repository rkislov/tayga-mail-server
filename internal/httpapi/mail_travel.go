package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/travel"
)

var pdfSlots = make(chan struct{}, 2)

type boundedPDFText struct{ data bytes.Buffer }

func (b *boundedPDFText) Write(p []byte) (int, error) {
	if b.data.Len()+len(p) > 512<<10 {
		return 0, errors.New("PDF text exceeds extraction limit")
	}
	return b.data.Write(p)
}
func pdfTicketText(ctx context.Context, data []byte) (string, error) {
	if len(data) > 8<<20 {
		return "", errors.New("PDF exceeds 8 MiB")
	}
	select {
	case pdfSlots <- struct{}{}:
		defer func() { <-pdfSlots }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", "-f", "1", "-l", "10", "-", "-")
	cmd.Stdin = bytes.NewReader(data)
	var out boundedPDFText
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.data.String(), nil
}
func (s *Server) mailTravelDrafts(w http.ResponseWriter, r *http.Request, au *authUser, id string) {
	msg, _, code := s.messageOwned(r, au, id)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": http.StatusText(code)})
		return
	}
	raw, err := s.ms.Read(msg.FilePath)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "read failed"})
		return
	}
	parsed := parseMIMEMessage(raw)
	text := parsed.Text
	if text == "" {
		text = mailsearch.ExtractHTMLText(parsed.HTML)
	}
	drafts := []*travel.Ticket{}
	warnings := []string{}
	if trip := travel.Parse(text); trip != nil {
		trip.Source = "message"
		drafts = append(drafts, trip)
	}
	PDFCount := 0
	for _, attachment := range parsed.Attachments {
		if attachment.ContentType != "application/pdf" && !strings.HasSuffix(strings.ToLower(attachment.Filename), ".pdf") {
			continue
		}
		PDFCount++
		if PDFCount > 4 {
			warnings = append(warnings, "Only the first four PDF attachments are processed")
			break
		}
		text, e := pdfTicketText(r.Context(), attachment.Data)
		if e != nil {
			warnings = append(warnings, attachment.Filename+": PDF extraction unavailable or failed; install poppler-utils (pdftotext)")
			continue
		}
		if trip := travel.Parse(text); trip != nil {
			trip.Source = attachment.Filename
			drafts = append(drafts, trip)
		}
	}
	writeJSON(w, 200, map[string]any{"trips": drafts, "warnings": warnings, "timezone_required": true})
}
