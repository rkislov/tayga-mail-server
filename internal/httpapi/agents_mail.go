package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
)

// Agent tools use the current authenticated user, never a provider/bot owner.
// Sending reuses the normal mail pipeline (sender checks, DLP, limits, queue).
func (s *Server) agentMailTool(ctx context.Context, u *storage.User, name string, args map[string]string) (any, error) {
	current, err := s.store.GetUserByID(ctx, u.ID)
	if err != nil || !current.Enabled || current.AuthSource == "bot" || current.AuthSource == "federated" || current.TenantID != u.TenantID || current.DomainID != u.DomainID {
		return nil, errors.New("access denied")
	}
	switch name {
	case "mail_search":
		query := mailsearch.ParseQuery(args["query"])
		hits, err := s.store.SearchMessages(ctx, u.ID, "", query.From, query.To, query.Subject, query.FTSQuery(), 10)
		if err != nil {
			return nil, err
		}
		out := []map[string]string{}
		for _, hit := range hits {
			m := hit.Message
			box, e := s.store.GetMailboxByID(ctx, m.MailboxID)
			if e != nil || box.UserID != u.ID {
				continue
			}
			out = append(out, map[string]string{"id": m.ID, "subject": m.Subject, "from": m.FromAddr, "to": m.ToAddr, "date": m.DateHdr, "snippet": agentMailClip(hit.Snippet, 3000)})
		}
		return out, nil
	case "mail_read":
		m, err := s.store.GetMessageByID(ctx, args["message_id"])
		if err != nil {
			return nil, err
		}
		box, err := s.store.GetMailboxByID(ctx, m.MailboxID)
		if err != nil || box.UserID != u.ID || m.Size > 1<<20 {
			return nil, errors.New("access denied or message too large")
		}
		raw, err := s.ms.Read(m.FilePath)
		if err != nil {
			return nil, err
		}
		if len(raw) > 1<<20 {
			return nil, errors.New("message too large")
		}
		parsed := parseMIMEMessage(raw)
		if parsed.Text == "" && parsed.HTML != "" {
			parsed.Text = mailsearch.ExtractHTMLText(parsed.HTML)
		}
		return map[string]string{"id": m.ID, "subject": parsed.Subject, "from": parsed.From, "to": parsed.To, "date": parsed.Date, "text": agentMailClip(parsed.Text, 20000)}, nil
	case "mail_send":
		data, _ := json.Marshal(map[string]any{"from": current.Email, "to": []string{args["to"]}, "subject": args["subject"], "text": args["text"]})
		req, err := http.NewRequestWithContext(ctx, "POST", "/api/v1/mail/send", bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		s.mailSend(response, req, &authUser{ID: current.ID, Email: current.Email})
		if response.Code < 200 || response.Code >= 300 {
			return nil, errors.New("mail delivery rejected")
		}
		return map[string]string{"status": "accepted"}, nil
	}
	return nil, errors.New("unknown mail tool")
}
func agentMailClip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
