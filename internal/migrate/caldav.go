package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/tayga/tms/internal/storage"
)

type davOptions struct {
	SkipTLSVerify bool   `json:"skip_tls_verify"`
	CalendarPath  string `json:"calendar_path"` // optional remote path filter
	LocalName     string `json:"local_name"`    // local calendar name; default "default"
}

func (s *Service) runCalDAV(ctx context.Context, job *Job) error {
	password, err := decryptPassword(s.key, job.PasswordCiphertext)
	if err != nil {
		return err
	}
	user, err := s.store.GetUserByID(ctx, job.UserID)
	if err != nil {
		return err
	}
	if err := s.store.EnsureDAVDefaults(ctx, user.ID); err != nil {
		return err
	}

	var opts davOptions
	_ = json.Unmarshal([]byte(job.Options), &opts)
	localName := strings.TrimSpace(opts.LocalName)
	if localName == "" {
		localName = "default"
	}

	baseClient, err := davHTTPClient(job.URL, opts.SkipTLSVerify)
	if err != nil {
		return err
	}
	defer baseClient.Transport.(interface{ CloseIdleConnections() }).CloseIdleConnections()
	httpClient := webdav.HTTPClientWithBasicAuth(baseClient, job.Username, password)
	cli, err := caldav.NewClient(httpClient, job.URL)
	if err != nil {
		return err
	}
	// A supplied collection URL must import that collection, not every calendar in its home.
	remote, _ := url.Parse(job.URL)
	cals, discoverErr := cli.FindCalendars(ctx, remote.Path)
	selected := []caldav.Calendar{}
	for _, calendar := range cals {
		if strings.TrimRight(calendar.Path, "/") == strings.TrimRight(remote.Path, "/") {
			selected = append(selected, calendar)
		}
	}
	if len(selected) > 0 {
		cals = selected
	} else if discoverErr != nil || len(cals) == 0 {
		principal, err := cli.FindCurrentUserPrincipal(ctx)
		if err != nil {
			return fmt.Errorf("principal: %w", err)
		}
		home, err := cli.FindCalendarHomeSet(ctx, principal)
		if err != nil {
			return fmt.Errorf("calendar-home: %w", err)
		}
		cals, err = cli.FindCalendars(ctx, home)
		if err != nil {
			return fmt.Errorf("find calendars: %w", err)
		}
	}
	if len(cals) == 0 {
		return fmt.Errorf("no calendars found at the source URL")
	}

	dbCal, err := s.store.EnsureCalendar(ctx, user.ID, localName, localName)
	if err != nil {
		return err
	}

	copied, skipped, errs := job.Copied, job.Skipped, job.Errors
	query := &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{
			Name: "VCALENDAR", AllProps: true, AllComps: true,
		},
		CompFilter: caldav.CompFilter{Name: "VCALENDAR", Comps: []caldav.CompFilter{{Name: "VEVENT"}}},
	}

	for _, cal := range cals {
		if opts.CalendarPath != "" && !strings.Contains(cal.Path, opts.CalendarPath) {
			continue
		}
		if ctx.Err() != nil || s.isCancelled(ctx, job.ID) {
			job.Copied, job.Skipped, job.Errors = copied, skipped, errs
			return ctx.Err()
		}
		objs, err := cli.QueryCalendar(ctx, cal.Path, query)
		// SOGo needs an explicit component filter; import tasks separately as well.
		if err == nil {
			tasksQuery := *query
			tasksQuery.CompFilter = caldav.CompFilter{Name: "VCALENDAR", Comps: []caldav.CompFilter{{Name: "VTODO"}}}
			tasks, tasksErr := cli.QueryCalendar(ctx, cal.Path, &tasksQuery)
			if tasksErr == nil {
				objs = append(objs, tasks...)
			} else {
				errs++
				_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, tasksErr.Error(), "")
			}
		}
		if err != nil {
			errs++
			_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, err.Error(), `{"cal":`+jsonString(cal.Path)+`}`)
			continue
		}
		for _, obj := range objs {
			if ctx.Err() != nil || s.isCancelled(ctx, job.ID) {
				job.Copied, job.Skipped, job.Errors = copied, skipped, errs
				return ctx.Err()
			}
			if obj.Data == nil {
				continue
			}
			compType, uid, verr := caldav.ValidateCalendarObject(obj.Data)
			if verr != nil || uid == "" {
				errs++
				continue
			}
			exists, err := s.store.CalendarObjectExistsByUID(ctx, dbCal.ID, uid)
			if err == nil && exists {
				skipped++
				continue
			}
			var buf bytes.Buffer
			if err := ical.NewEncoder(&buf).Encode(obj.Data); err != nil {
				errs++
				continue
			}
			href := path.Base(strings.TrimSuffix(obj.Path, "/"))
			if href == "" || href == "." || href == "/" {
				href = uid + ".ics"
			}
			dtStart, dtEnd := extractEventTimes(obj.Data)
			_, err = s.store.UpsertCalendarObject(ctx, &storage.CalendarObject{
				CalendarID: dbCal.ID,
				UID:        uid,
				HrefName:   href,
				Data:       buf.String(),
				Component:  compType,
				DTStart:    dtStart,
				DTEnd:      dtEnd,
			})
			if err != nil {
				errs++
				continue
			}
			copied++
			if (copied+skipped+errs)%20 == 0 {
				_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, "", `{"cal":`+jsonString(cal.Path)+`}`)
			}
		}
		_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, "", `{"cal":`+jsonString(cal.Path)+`}`)
	}
	job.Copied, job.Skipped, job.Errors = copied, skipped, errs
	_ = s.bumpProgress(ctx, job.ID, copied, skipped, errs, "", "{}")
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
		break
	}
	return start, end
}
