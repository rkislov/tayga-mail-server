package httpapi

import (
	"context"
	"fmt"
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
			s.log.Warn("calendar delivery queue", "error", err)
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
			s.log.Warn("calendar delivery retry", "delivery_id", job.ID, "attempt", job.Attempts, "phase", job.State, "error", err)
			if retryErr := s.store.RetryCalendarDelivery(ctx, job, err.Error()); retryErr != nil {
				return retryErr
			}
		}
	}
	return nil
}

func (s *Server) deliverCalendarJob(ctx context.Context, job *storage.CalendarDelivery) error {
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
				return fmt.Errorf("calendar recipient disabled")
			}
			mb, err := s.store.EnsureMailbox(ctx, user.ID, "INBOX", s.ms.UserRoot(user.Email))
			if err != nil {
				return err
			}
			rel, size, err := s.ms.Deliver(user.Email, "INBOX", job.Data)
			if err != nil {
				return err
			}
			message = &storage.Message{MailboxID: mb.ID, Size: size, FilePath: rel, InternalDate: time.Unix(0, job.CreatedAt).UTC()}
			mailsearch.ApplyHeaders(message, job.Data)
		}
		if err := s.store.CommitCalendarDelivery(ctx, job, message); err != nil {
			// A failed database connection can leave the commit outcome unknown.
			// Keep the file: deleting it could corrupt an already committed mail.
			return err
		}
		job.State = "notifying"
		s.log.Info("calendar mail handed off", "delivery_id", job.ID, "local", job.RecipientUserID != "")
	}
	if job.RecipientUserID != "" {
		if _, err := s.store.GetMessageByID(ctx, job.ID); err == nil {
			if err = mailsearch.Index(ctx, s.store, job.ID, job.Data); err != nil {
				return err
			}
		} else if err != storage.ErrNotFound {
			return err
		}
		if s.notify != nil {
			ev := notify.Event{ID: job.ID, Kind: notify.KindCalendar, Title: job.Subject, Body: job.Sender, Href: "calendar", Mailbox: "INBOX", CreatedAt: time.Unix(0, job.CreatedAt).UTC()}
			if err := s.notify.Publish(job.RecipientUserID, ev); err != nil {
				return err
			}
		}
	}
	return s.store.CompleteCalendarDelivery(ctx, job)
}
