package dav

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/tayga/tms/internal/storage"
)

type calBackend struct {
	store storage.Driver
}

func (b *calBackend) CurrentUserPrincipal(ctx context.Context) (string, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return "", webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	return calPrincipal(u.Email), nil
}

func (b *calBackend) CalendarHomeSetPath(ctx context.Context) (string, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return "", webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	return calHome(u.Email), nil
}

func (b *calBackend) CreateCalendar(ctx context.Context, calendar *caldav.Calendar) error {
	u, ok := userFrom(ctx)
	if !ok {
		return webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, name, href, okParse := parseCalPath(calendar.Path)
	if !okParse || href != "" || name == "" {
		return webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("invalid calendar path"))
	}
	if !strings.EqualFold(email, u.Email) {
		return webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	display := calendar.Name
	if display == "" {
		display = name
	}
	_, err := b.store.CreateCalendar(ctx, &storage.Calendar{
		UserID:      u.ID,
		Name:        name,
		DisplayName: display,
		Description: calendar.Description,
	})
	if err != nil {
		return webdav.NewHTTPError(http.StatusConflict, err)
	}
	return nil
}

func (b *calBackend) ListCalendars(ctx context.Context) ([]caldav.Calendar, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	rows, err := b.store.ListCalendars(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	out := make([]caldav.Calendar, 0, len(rows))
	for _, c := range rows {
		out = append(out, toCalDAVCalendar(u.Email, c))
	}
	return out, nil
}

func (b *calBackend) GetCalendar(ctx context.Context, p string) (*caldav.Calendar, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, name, href, okParse := parseCalPath(p)
	if !okParse || name == "" || href != "" {
		return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found"))
	}
	if !strings.EqualFold(email, u.Email) {
		return nil, webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	c, err := b.store.GetCalendarByName(ctx, u.ID, name)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusNotFound, err)
	}
	cal := toCalDAVCalendar(u.Email, c)
	return &cal, nil
}

func (b *calBackend) GetCalendarObject(ctx context.Context, p string, _ *caldav.CalendarCompRequest) (*caldav.CalendarObject, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, calName, href, okParse := parseCalPath(p)
	if !okParse || calName == "" || href == "" {
		return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found"))
	}
	if !strings.EqualFold(email, u.Email) {
		return nil, webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	cal, err := b.store.GetCalendarByName(ctx, u.ID, calName)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusNotFound, err)
	}
	obj, err := b.store.GetCalendarObject(ctx, cal.ID, href)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusNotFound, err)
	}
	return toCalDAVObject(u.Email, calName, obj)
}

func (b *calBackend) ListCalendarObjects(ctx context.Context, p string, _ *caldav.CalendarCompRequest) ([]caldav.CalendarObject, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, calName, href, okParse := parseCalPath(p)
	if !okParse || calName == "" || href != "" {
		return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found"))
	}
	if !strings.EqualFold(email, u.Email) {
		return nil, webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	cal, err := b.store.GetCalendarByName(ctx, u.ID, calName)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusNotFound, err)
	}
	rows, err := b.store.ListCalendarObjects(ctx, cal.ID)
	if err != nil {
		return nil, err
	}
	out := make([]caldav.CalendarObject, 0, len(rows))
	for _, row := range rows {
		co, err := toCalDAVObject(u.Email, calName, row)
		if err != nil {
			return nil, err
		}
		out = append(out, *co)
	}
	return out, nil
}

func (b *calBackend) QueryCalendarObjects(ctx context.Context, p string, query *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	objs, err := b.ListCalendarObjects(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	return caldav.Filter(query, objs)
}

func (b *calBackend) PutCalendarObject(ctx context.Context, p string, cal *ical.Calendar, opts *caldav.PutCalendarObjectOptions) (*caldav.CalendarObject, error) {
	u, ok := userFrom(ctx)
	if !ok {
		return nil, webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, calName, href, okParse := parseCalPath(p)
	if !okParse || calName == "" {
		return nil, webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("invalid path"))
	}
	if !strings.EqualFold(email, u.Email) {
		return nil, webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	if href == "" {
		href = storage.NewID() + ".ics"
	}
	compType, uid, err := caldav.ValidateCalendarObject(cal)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusBadRequest, err)
	}
	if uid == "" {
		uid = storage.NewID()
	}

	dbCal, err := b.store.GetCalendarByName(ctx, u.ID, calName)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusNotFound, err)
	}
	existing, err := b.store.GetCalendarObject(ctx, dbCal.ID, href)
	if err != nil && err != storage.ErrNotFound {
		return nil, err
	}
	if err == storage.ErrNotFound {
		existing = nil
	}
	if err := checkCalConditions(opts, existing); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return nil, webdav.NewHTTPError(http.StatusBadRequest, err)
	}
	dtStart, dtEnd := extractEventTimes(cal)
	saved, err := b.store.UpsertCalendarObject(ctx, &storage.CalendarObject{
		CalendarID: dbCal.ID,
		UID:        uid,
		HrefName:   href,
		Data:       buf.String(),
		Component:  compType,
		DTStart:    dtStart,
		DTEnd:      dtEnd,
	})
	if err != nil {
		return nil, err
	}
	return toCalDAVObject(u.Email, calName, saved)
}

func (b *calBackend) DeleteCalendarObject(ctx context.Context, p string) error {
	u, ok := userFrom(ctx)
	if !ok {
		return webdav.NewHTTPError(http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	}
	email, calName, href, okParse := parseCalPath(p)
	if !okParse || calName == "" || href == "" {
		return webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found"))
	}
	if !strings.EqualFold(email, u.Email) {
		return webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("forbidden"))
	}
	cal, err := b.store.GetCalendarByName(ctx, u.ID, calName)
	if err != nil {
		return webdav.NewHTTPError(http.StatusNotFound, err)
	}
	if err := b.store.DeleteCalendarObject(ctx, cal.ID, href); err != nil {
		return webdav.NewHTTPError(http.StatusNotFound, err)
	}
	return nil
}

func toCalDAVCalendar(email string, c *storage.Calendar) caldav.Calendar {
	return caldav.Calendar{
		Path:                  calCollection(email, c.Name),
		Name:                  c.DisplayName,
		Description:           c.Description,
		SupportedComponentSet: []string{"VEVENT", "VTODO", "VJOURNAL"},
	}
}

func toCalDAVObject(email, calName string, o *storage.CalendarObject) (*caldav.CalendarObject, error) {
	cal, err := ical.NewDecoder(strings.NewReader(o.Data)).Decode()
	if err != nil {
		return nil, err
	}
	return &caldav.CalendarObject{
		Path:          calObjectPath(email, calName, o.HrefName),
		ModTime:       o.UpdatedAt,
		ContentLength: o.Size,
		ETag:          o.ETag,
		Data:          cal,
	}, nil
}

func checkCalConditions(opts *caldav.PutCalendarObjectOptions, existing *storage.CalendarObject) error {
	if opts == nil {
		return nil
	}
	if opts.IfNoneMatch.IsSet() {
		if opts.IfNoneMatch.IsWildcard() && existing != nil {
			return webdav.NewHTTPError(http.StatusPreconditionFailed, fmt.Errorf("resource exists"))
		}
	}
	if opts.IfMatch.IsSet() {
		if existing == nil {
			return webdav.NewHTTPError(http.StatusPreconditionFailed, fmt.Errorf("resource missing"))
		}
		ok, err := opts.IfMatch.MatchETag(existing.ETag)
		if err != nil || !ok {
			return webdav.NewHTTPError(http.StatusPreconditionFailed, fmt.Errorf("etag mismatch"))
		}
	}
	return nil
}

func extractEventTimes(cal *ical.Calendar) (start, end *time.Time) {
	for _, comp := range cal.Children {
		if comp.Name != ical.CompEvent && comp.Name != ical.CompToDo {
			continue
		}
		if prop := comp.Props.Get(ical.PropDateTimeStart); prop != nil {
			if t, err := prop.DateTime(time.UTC); err == nil {
				start = &t
			}
		}
		if prop := comp.Props.Get(ical.PropDateTimeEnd); prop != nil {
			if t, err := prop.DateTime(time.UTC); err == nil {
				end = &t
			}
		} else if prop := comp.Props.Get(ical.PropDuration); prop != nil && start != nil {
			if d, err := prop.Duration(); err == nil {
				t := start.Add(d)
				end = &t
			}
		}
		return start, end
	}
	return nil, nil
}
