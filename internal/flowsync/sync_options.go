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
	Filter int
	Bodies []bodyPreference
}

func (p syncPreferences) hash() string { raw, _ := json.Marshal(p); return string(raw) }
func preferencesFrom(request *protocolNode, previous syncPreferences) (syncPreferences, error) {
	options := request.child("Options")
	if options == nil {
		return previous, nil
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
		if p.Type != "1" && p.Type != "2" {
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
		preference = p.Bodies[0]
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
	size := len([]byte(text))
	limit := 32 << 10
	if preference.HasSize {
		limit = preference.Size
	}
	if limit > 1<<20 {
		limit = 1 << 20
	}
	truncated := size > limit
	if truncated {
		if preference.AllOrNone {
			text = ""
		} else {
			end := limit
			for end > 0 && !utf8.ValidString(text[:end]) {
				end--
			}
			text = text[:end]
		}
	}
	return text, kind, size, truncated
}
