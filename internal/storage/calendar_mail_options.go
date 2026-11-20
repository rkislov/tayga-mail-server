// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"
	"github.com/tayga/tms/internal/calutil"
)

type calendarMailOptionsKey struct{}
type calendarMailOptions struct{ SaveCopy, ChangedOnly bool }

// WithCalendarMailOptions selects EWS delivery semantics without changing the
// transaction boundary used by other calendar protocols.
func WithCalendarMailOptions(ctx context.Context, saveCopy, changedOnly bool) context.Context {
	return context.WithValue(ctx, calendarMailOptionsKey{}, calendarMailOptions{saveCopy, changedOnly})
}
func calendarMailChangedOnly(ctx context.Context) bool {
	opts, _ := ctx.Value(calendarMailOptionsKey{}).(calendarMailOptions)
	return opts.ChangedOnly
}

// MS-OXWSMTGS: Changed means addresses added/deleted, not a subject/time edit.
// Evaluate membership from the two snapshots protected by the calendar lock.
func filterChangedCalendarMail(ctx context.Context, mails []CalendarMail, current, previous string) ([]CalendarMail, error) {
	if !calendarMailChangedOnly(ctx) {
		return mails, nil
	}
	added, err := calutil.SchedulingAttendees(current)
	if err != nil {
		return nil, err
	}
	var removed []calutil.Attendee
	if previous != "" {
		removed, err = calutil.SchedulingAttendees(previous)
		if err != nil {
			return nil, err
		}
	}
	changed := map[string]bool{}
	for _, list := range [][]calutil.Attendee{added, removed} {
		seen := map[string]bool{}
		for _, a := range list {
			key := calendarMailAddress(a.Email)
			if !seen[key] {
				changed[key] = !changed[key]
				seen[key] = true
			}
		}
	}
	var out []CalendarMail
	for _, m := range mails {
		if changed[calendarMailAddress(m.To)] {
			out = append(out, m)
		}
	}
	return out, nil
}
