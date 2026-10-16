// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tayga/tms/internal/mailsearch"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type preferencesKey struct{}
type deleteMovesKey struct{}
type bodyPreference struct {
	Type      string
	Size      int
	HasSize   bool
	AllOrNone bool
}
type syncPreferences struct {
	Filter      int
	Bodies      []bodyPreference
	MIMESupport int `json:",omitempty"`
}

func (p syncPreferences) hash() string { raw, _ := json.Marshal(p); return string(raw) }
func preferencesFrom(request *protocolNode, previous syncPreferences) (syncPreferences, error) {
	options := request.child("Options")
	if options == nil {
		return previous, nil
	}
	if node := options.child("MIMESupport"); node != nil {
		n, err := strconv.Atoi(strings.TrimSpace(node.Text))
		if err != nil || n < 0 || n > 2 {
			return previous, fmt.Errorf("invalid MIME support")
		}
		previous.MIMESupport = n
	}
	if f := options.child("FilterType"); f != nil {
		n, err := strconv.Atoi(strings.TrimSpace(f.Text))
		if err != nil || n < 0 || n > 8 {
			return previous, fmt.Errorf("invalid filter")
		}
		previous.Filter = n
	}
	var bodies []bodyPreference
	for _, node := range options.Children {
		if node.Name.Local != "BodyPreference" {
			continue
		}
		p := bodyPreference{Type: node.value("Type")}
		if p.Type != "1" && p.Type != "2" && p.Type != "4" {
			continue
		}
		if n := node.child("TruncationSize"); n != nil {
			size, err := strconv.Atoi(strings.TrimSpace(n.Text))
			if err != nil || size < 0 {
				return previous, fmt.Errorf("invalid truncation")
			}
			p.Size = size
			p.HasSize = true
		}
		p.AllOrNone = node.value("AllOrNone") == "1"
		bodies = append(bodies, p)
	}
	if len(bodies) > 0 {
		previous.Bodies = bodies
	}
	return previous, nil
}
func filterCutoff(filter int, now time.Time) time.Time {
	switch filter {
	case 1:
		return now.AddDate(0, 0, -1)
	case 2:
		return now.AddDate(0, 0, -3)
	case 3:
		return now.AddDate(0, 0, -7)
	case 4:
		return now.AddDate(0, 0, -14)
	case 5:
		return now.AddDate(0, -1, 0)
	case 6:
		return now.AddDate(0, -3, 0)
	case 7:
		return now.AddDate(0, -6, 0)
	}
	return time.Time{}
}
func contextPreferences(ctx context.Context) syncPreferences {
	p, _ := ctx.Value(preferencesKey{}).(syncPreferences)
	return p
}
func validFilter(kind collectionKind, p syncPreferences) bool {
	switch kind {
	case kindMail:
		return p.Filter <= 5
	case kindCalendar:
		return p.Filter == 0 || (p.Filter >= 4 && p.Filter <= 7)
	}
	return true
}

func preferredBody(content messageContent, p syncPreferences) (string, string, int, bool) {
	kind, text := "1", content.Text
	var preference bodyPreference
	if len(p.Bodies) > 0 {
		// Prefer the native body among the formats accepted by the client.
		// MIME is only returned when explicitly enabled and requested.
		for _, candidate := range p.Bodies {
			if candidate.Type == "4" {
				if len(content.MIME) > 0 && (p.MIMESupport == 2 || (p.MIMESupport == 1 && content.SMIME)) {
					preference = candidate
					break
				}
				continue
			}
			if preference.Type == "" || candidate.Type == content.nativeBodyType() {
				preference = candidate
			}
		}
		if preference.Type == "" {
			preference = bodyPreference{Type: "1", Size: 32 << 10, HasSize: true}
		}
		kind = preference.Type
	} else if content.HTML != "" {
		kind = "2"
	}
	if kind == "2" {
		text = content.HTML
		if text == "" {
			kind = "1"
			text = content.Text
		}
	}
	if kind == "1" && text == "" && content.HTML != "" {
		text = mailsearch.ExtractHTMLText(content.HTML)
	}
	if kind == "4" {
		text = string(content.MIME)
	}
	size := len([]byte(text))
	limit := 32 << 10
	// An explicit BodyPreference without TruncationSize requests the whole
	// body, including when Apple Mail fetches a previously truncated message.
	if len(p.Bodies) > 0 && !preference.HasSize {
		limit = size
	}
	if preference.HasSize {
		limit = preference.Size
	}
	truncated := size > limit
	if truncated {
		if preference.AllOrNone {
			text = ""
		} else {
			end := limit
			for kind != "4" && end > 0 && !utf8.ValidString(text[:end]) {
				end--
			}
			text = text[:end]
		}
	}
	return text, kind, size, truncated
}
