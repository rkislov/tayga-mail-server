// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

type easRecipient struct {
	email, name string
	kind        int
	user        *storage.User
}

func (h *easHandler) recipientDirectory(ctx context.Context, u *storage.User) (map[string]easRecipient, error) {
	allowed, err := (&ewsHandler{store: h.store}).availabilityUsers(ctx, u)
	if err != nil {
		return nil, err
	}
	pool := map[string]easRecipient{}
	for email, peer := range allowed {
		pool[email] = easRecipient{email, fieldOr(peer.DisplayName, email), 1, peer}
	}
	books, err := h.store.ListAddressBooks(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	for _, book := range books {
		cards, err := h.store.ListAddressObjects(ctx, book.ID)
		if err != nil {
			return nil, err
		}
		for _, card := range cards {
			c := parseVCard(card.Data)
			for _, address := range []string{c.Email, c.Email2} {
				email, err := calendarEmail(address)
				if err != nil {
					continue
				}
				if _, exists := pool[email]; !exists {
					pool[email] = easRecipient{email, fieldOr(c.FN, email), 2, nil}
				}
			}
		}
	}
	return pool, nil
}

// ResolveRecipients is also the iOS Calendar free/busy endpoint (MS-ASCMD
// 2.2.1.15). Unavailable/unauthorized calendars never look like free calendars.
func (h *easHandler) resolveRecipients(ctx context.Context, u *storage.User, body []byte) (string, error) {
	result := func(status int) string {
		return fmt.Sprintf(`<ResolveRecipients xmlns="ResolveRecipients:"><Status>%d</Status></ResolveRecipients>`, status)
	}
	doc, err := parseProtocolXML(body)
	if err != nil {
		return result(5), nil
	}
	req := doc.child("ResolveRecipients")
	if req == nil {
		return result(5), nil
	}
	var addresses []string
	for _, n := range req.Children {
		if n.Name.Local == "To" {
			if strings.TrimSpace(n.Text) == "" || len(n.Text) > 1024 {
				return result(5), nil
			}
			addresses = append(addresses, n.Text)
		}
	}
	if len(addresses) == 0 || len(addresses) > 100 {
		return result(5), nil
	}
	options := req.child("Options")
	maxAmbiguous := 0
	certificates := 0
	if raw := options.value("MaxAmbiguousRecipients"); raw != "" {
		maxAmbiguous, err = strconv.Atoi(raw)
		if err != nil || maxAmbiguous < 0 || maxAmbiguous > 9999 {
			return result(5), nil
		}
	}
	if maxAmbiguous > 100 {
		maxAmbiguous = 100
	}
	if raw := options.value("CertificateRetrieval"); raw != "" {
		certificates, err = strconv.Atoi(raw)
		if err != nil || certificates < 1 || certificates > 3 {
			return result(5), nil
		}
	}
	availability := options.child("Availability")
	var from, to time.Time
	if availability != nil {
		from, err = time.Parse(time.RFC3339Nano, availability.value("StartTime"))
		if err != nil {
			return result(5), nil
		}
		to = from.Add(7 * 24 * time.Hour)
		if raw := availability.value("EndTime"); raw != "" {
			to, err = time.Parse(time.RFC3339Nano, raw)
		}
		if err != nil || to.Sub(from) < 30*time.Minute || to.Sub(from) > 62*24*time.Hour {
			return result(5), nil
		}
	}
	pool, err := h.recipientDirectory(ctx, u)
	if err != nil {
		return result(6), nil
	}
	var out strings.Builder
	out.WriteString(`<ResolveRecipients xmlns="ResolveRecipients:"><Status>1</Status>`)
	for _, raw := range addresses {
		query := strings.ToLower(strings.TrimSpace(raw))
		var matches []easRecipient
		if exact, ok := pool[query]; ok {
			matches = append(matches, exact)
		} else {
			for _, candidate := range pool {
				if strings.Contains(candidate.email, query) || strings.Contains(strings.ToLower(candidate.name), query) {
					matches = append(matches, candidate)
				}
			}
		}
		sort.Slice(matches, func(i, j int) bool { return matches[i].email < matches[j].email })
		status, count := 1, len(matches)
		if count == 0 {
			status = 4
		} else if count > 1 {
			status = 2
			if count > maxAmbiguous {
				status = 3
				matches = matches[:maxAmbiguous]
			}
		}
		fmt.Fprintf(&out, `<Response><To>%s</To><Status>%d</Status><RecipientCount>%d</RecipientCount>`, xmlEscape(raw), status, count)
		for _, peer := range matches {
			fmt.Fprintf(&out, `<Recipient><Type>%d</Type><DisplayName>%s</DisplayName><EmailAddress>%s</EmailAddress>`, peer.kind, xmlEscape(peer.name), xmlEscape(peer.email))
			if availability != nil && count == 1 {
				availabilityStatus, merged := 163, ""
				if peer.user != nil {
					busy, e := (&ewsHandler{store: h.store}).ownedCalendarBusy(ctx, peer.user, from, to)
					if e != nil {
						availabilityStatus = 162
					} else {
						availabilityStatus = 1
						merged = mergedEASBusy(busy, from, to)
					}
				}
				fmt.Fprintf(&out, `<Availability><Status>%d</Status>`, availabilityStatus)
				if availabilityStatus == 1 {
					out.WriteString(`<MergedFreeBusy>` + merged + `</MergedFreeBusy>`)
				}
				out.WriteString(`</Availability>`)
			}
			if certificates > 1 && count == 1 {
				out.WriteString(`<Certificates><Status>7</Status><CertificateCount>0</CertificateCount></Certificates>`)
			}
			if options.child("Picture") != nil {
				out.WriteString(`<Picture><Status>173</Status></Picture>`)
			}
			out.WriteString(`</Recipient>`)
		}
		out.WriteString(`</Response>`)
	}
	out.WriteString(`</ResolveRecipients>`)
	return out.String(), nil
}

func mergedEASBusy(busy []storage.BusyInterval, from, to time.Time) string {
	const step = 30 * time.Minute
	merged := []byte(strings.Repeat("0", int((to.Sub(from)+step-1)/step)))
	for _, b := range busy {
		start, end := b.Start, b.End
		if start.Before(from) {
			start = from
		}
		if end.After(to) {
			end = to
		}
		if !end.After(start) {
			continue
		}
		for i := int(start.Sub(from) / step); i < len(merged) && from.Add(time.Duration(i)*step).Before(end); i++ {
			merged[i] = '2'
		}
	}
	return string(merged)
}
