package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/notify"
	"github.com/tayga/tms/internal/storage"
)

func (s *Server) runCalendarDeliveries(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.drainCalendarDeliveries(ctx); err != nil && ctx.Err() == nil {
			s.log.Warn("local delivery queue", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) drainCalendarDeliveries(ctx context.Context) error {
	for i := 0; i < 32; i++ {
		job, err := s.store.ClaimCalendarDelivery(ctx)
		if err == storage.ErrNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		// Stay within the claim lease. The token check fences late workers too.
		jobCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		err = s.deliverCalendarJob(jobCtx, job)
		cancel()
		if err != nil {
			s.log.Warn("mail delivery retry", "kind", job.Kind, "delivery_id", job.ID, "attempt", job.Attempts, "phase", job.State, "error", err)
			if retryErr := s.store.RetryCalendarDelivery(ctx, job, err.Error()); retryErr != nil {
				return retryErr
			}
		}
	}
	return nil
}

func (s *Server) deliverCalendarJob(ctx context.Context, job *storage.CalendarDelivery) error {
	if job.Kind == "welcome" && job.RecipientUserID == "" {
		return storage.ErrUnauthorized
	}
	if job.State == "pending" {
		var message *storage.Message
		if job.RecipientUserID != "" {
			if s.ms == nil {
				return fmt.Errorf("mail storage unavailable")
			}
			user, err := s.store.GetUserByID(ctx, job.RecipientUserID)
			if err != nil {
				return err
			}
			if !user.Enabled {
				return fmt.Errorf("mail recipient disabled")
			}
			if job.Kind == "welcome" {
				if int64(len(job.Data)) > s.mailLimits((&http.Request{}).WithContext(ctx), user)["message_bytes"] {
					return fmt.Errorf("welcome mail exceeds the recipient message size limit")
				}
				if user.QuotaBytes > 0 {
					used, err := s.store.SumMailboxBytes(ctx, user.ID)
					if err != nil {
						return err
					}
					if int64(len(job.Data)) > user.QuotaBytes-used {
						return storage.ErrQuotaExceeded
					}
				}
			}
			folder, path := "INBOX", s.ms.UserRoot(user.Email)
			if job.TargetMailbox == "Sent" {
				folder = "Sent"
				path, err = s.ms.EnsureFolder(user.Email, folder)
				if err != nil {
					return err
				}
			}
			mb, err := s.store.EnsureMailbox(ctx, user.ID, folder, path)
			if err != nil {
				return err
			}
			build := func() (*storage.Message, error) {
				var rel string
				var size int64
				var err error
				if job.Kind == "welcome" {
					rel, size, err = s.ms.DeliverOnce(user.Email, folder, job.ID, job.Data)
				} else {
					rel, size, err = s.ms.Deliver(user.Email, folder, job.Data)
				}
				if err != nil {
					return nil, err
				}
				message := &storage.Message{MailboxID: mb.ID, Size: size, FilePath: rel, InternalDate: time.Unix(0, job.CreatedAt).UTC()}
				mailsearch.ApplyHeaders(message, job.Data)
				return message, nil
			}
			if job.Kind == "welcome" {
				if err := s.store.CommitWelcomeDelivery(ctx, job, build); err != nil {
					return err
				}
			} else {
				message, err = build()
				if err != nil {
					return err
				}
			}
		}
		if job.Kind != "welcome" {
			if err := s.store.CommitCalendarDelivery(ctx, job, message); err != nil {
				// The outcome of a disconnected commit can be unknown; retain the file.
				return err
			}
		}
		job.State = "notifying"
		s.log.Info("mail handed off", "kind", job.Kind, "delivery_id", job.ID, "local", job.RecipientUserID != "", "origin", job.OriginJSON)
	}
	if job.RecipientUserID != "" {
		if _, err := s.store.GetMessageByID(ctx, job.ID); err == nil {
			if err = mailsearch.Index(ctx, s.store, job.ID, job.Data); err != nil {
				return err
			}
		} else if err != storage.ErrNotFound {
			return err
		}
		if s.notify != nil && job.TargetMailbox != "Sent" {
			ev := notify.Event{ID: job.ID, Kind: notify.KindCalendar, Title: job.Subject, Body: job.Sender, Href: "calendar", Mailbox: "INBOX", CreatedAt: time.Unix(0, job.CreatedAt).UTC()}
			if job.Kind == "welcome" {
				ev.Kind = notify.KindMail
				ev.Href = "mail"
			}
			if err := s.notify.Publish(job.RecipientUserID, ev); err != nil {
				return err
			}
		}
	}
	return s.store.CompleteCalendarDelivery(ctx, job)
}
