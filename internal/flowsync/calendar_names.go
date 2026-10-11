// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

type protocolLanguageKey struct{}

// Localize only generated names of standard collections. User-defined names and
// stable collection IDs are preserved, so localization does not create calendars.
func calendarClientName(ctx context.Context, cal *storage.Calendar) string {
	defaults := map[string]struct {
		ru, en  string
		aliases []string
	}{
		"default":  {"Основной календарь", "Main calendar", []string{"Календарь", "Calendar", "Основной календарь", "Main calendar"}},
		"personal": {"Личный календарь", "Personal calendar", []string{"Personal calendar", "Личный календарь"}},
		"work":     {"Рабочий календарь", "Work calendar", []string{"Work calendar", "Рабочий календарь"}},
	}
	entry, standard := defaults[cal.Name]
	generated := cal.DisplayName == ""
	for _, alias := range entry.aliases {
		if cal.DisplayName == alias {
			generated = true
		}
	}
	if standard && generated {
		language, _ := ctx.Value(protocolLanguageKey{}).(string)
		if strings.HasPrefix(strings.ToLower(language), "en") {
			return entry.en
		}
		return entry.ru
	}
	if cal.DisplayName != "" {
		return cal.DisplayName
	}
	return cal.Name
}
