package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/tayga/tms/internal/storage"
)

type davOptions struct {
	CalendarPath string `json:"calendar_path"` // optional remote path filter
	LocalName    string `json:"local_name"`    // local calendar name; default "default"
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

	httpClient := webdav.HTTPClientWithBasicAuth(&http.Client{Timeout: 60 * time.Second}, job.Username, password)
	cli, err := caldav.NewClient(httpClient, job.URL)
	if err != nil {
		return err
	}
	principal, err := cli.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return fmt.Errorf("principal: %w", err)
	}
	home, err := cli.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		return fmt.Errorf("calendar-home: %w", err)
	}
	cals, err := cli.FindCalendars(ctx, home)
	if err != nil {
		return fmt.Errorf("find calendars: %w", err)
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
		CompFilter: caldav.CompFilter{Name: "VCALENDAR"},
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
