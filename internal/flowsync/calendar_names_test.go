// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestCalendarClientNamesPreserveCustomNames(t *testing.T) {
	for _, test := range []struct{ name, display, language, want string }{
		{"default", "Календарь", "ru-RU", "Основной календарь"},
		{"personal", "Personal calendar", "ru-RU", "Личный календарь"},
		{"work", "Work calendar", "ru-RU", "Рабочий календарь"},
		{"personal", "Personal calendar", "", "Личный календарь"},
		{"default", "Календарь", "en-US", "Main calendar"},
		{"work", "Work calendar", "en-US", "Work calendar"},
		{"personal", "Семья", "en-US", "Семья"},
		{"other", "Work calendar", "ru-RU", "Work calendar"},
	} {
		ctx := context.WithValue(context.Background(), protocolLanguageKey{}, test.language)
		cal := &storage.Calendar{ID: "stable-id", Name: test.name, DisplayName: test.display}
		if got := calendarClientName(ctx, cal); got != test.want {
			t.Errorf("%+v: got %q", test, got)
		}
		if cal.ID != "stable-id" || cal.DisplayName != test.display {
			t.Fatal("changed stored collection")
		}
	}
}
