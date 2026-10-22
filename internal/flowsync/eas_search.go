// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

// Search supports the domain GAL used by iOS attendee autocomplete. Other
// stores return an explicit protocol error instead of an empty success.
func (h *easHandler) searchGAL(ctx context.Context, u *storage.User, body []byte) (string, error) {
	result := func(status int) string {
		return fmt.Sprintf(`<Search xmlns="Search:"><Status>1</Status><Response><Store><Status>%d</Status></Store></Response></Search>`, status)
	}
	doc, err := parseProtocolXML(body)
	if err != nil {
		return result(2), nil
	}
	req := doc.child("Search")
	store := req.child("Store")
	if store == nil || len(req.Children) != 1 || store.value("Name") != "GAL" {
		return result(2), nil
	}
	query := store.child("Query")
	if query == nil || len(query.Children) > 0 || len(query.Text) > 1024 {
		return result(2), nil
	}
	text := strings.ToLower(strings.TrimSpace(query.Text))
	from, to := 0, 99
	if raw := store.child("Options").value("Range"); raw != "" {
		parts := strings.Split(raw, "-")
		if len(parts) != 2 {
			return result(2), nil
		}
		var e1, e2 error
		from, e1 = strconv.Atoi(parts[0])
		to, e2 = strconv.Atoi(parts[1])
		if e1 != nil || e2 != nil || from < 0 || to < from || to-from > 99 || to > 100000 {
			return result(2), nil
		}
	}
	cards, err := (&ewsHandler{store: h.store}).galContacts(ctx, u)
	if err != nil && !errors.Is(err, storage.ErrUnauthorized) {
		return result(3), nil
	}
	var matches []vcardFields
	for _, card := range cards {
		v := parseVCard(card.Data)
		if strings.Contains(strings.ToLower(v.FN+" "+v.Email), text) {
			matches = append(matches, v)
		}
	}
	var out strings.Builder
	out.WriteString(`<Search xmlns="Search:"><Status>1</Status><Response><Store><Status>1</Status>`)
	end := min(to+1, len(matches))
	for i := from; i < end; i++ {
		v := matches[i]
		out.WriteString(`<Result><Properties>`)
		fmt.Fprintf(&out, `<g:DisplayName xmlns:g="GAL:">%s</g:DisplayName><g:EmailAddress xmlns:g="GAL:">%s</g:EmailAddress>`, xmlEscape(v.FN), xmlEscape(v.Email))
		out.WriteString(`</Properties></Result>`)
	}
	if end > from {
		fmt.Fprintf(&out, `<Range>%d-%d</Range>`, from, end-1)
	}
	fmt.Fprintf(&out, `<Total>%d</Total></Store></Response></Search>`, len(matches))
	return out.String(), nil
}
