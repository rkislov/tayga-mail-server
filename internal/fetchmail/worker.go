package fetchmail

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
)

func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}
	for i := 0; i < 2; i++ {
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.drain(ctx)
				}
			}
		}()
	}
}
func (s *Service) claim(ctx context.Context) (*Source, error) {
	now := time.Now().Unix()
	rows, err := s.db.QueryContext(ctx, s.query(`SELECT `+columns+` FROM mail_sources WHERE enabled=1 AND next_run<=? AND lease_until<=? ORDER BY next_run,created_at LIMIT 100`), now, now)
	if err != nil {
		return nil, err
	}
	candidates := []*Source{}
	for rows.Next() {
		a, e := scan(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		candidates = append(candidates, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var a *Source
	for _, candidate := range candidates {
		if s.Writers != nil && s.Writers.AllowWrite(ctx, candidate.UserID) != nil {
			continue
		}
		a = candidate
		break
	}
	if a == nil {
		return nil, nil
	}
	token := storage.NewID()
	result, err := s.db.ExecContext(ctx, s.query(`UPDATE mail_sources SET lease_until=?,lease_token=? WHERE id=? AND enabled=1 AND next_run<=? AND lease_until<=?`), now+600, token, a.ID, now, now)
	if err != nil {
		return nil, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return nil, nil
	}
	return scan(s.db.QueryRowContext(ctx, s.query(`SELECT `+columns+` FROM mail_sources WHERE id=? AND lease_token=?`), a.ID, token))
}
func (s *Service) drain(ctx context.Context) {
	a, err := s.claim(ctx)
	if err != nil {
		s.log.Warn("mail collection claim failed", "error", err)
		return
	}
	if a == nil {
		return
	}
	s.run(ctx, a)
}
func (s *Service) run(ctx context.Context, a *Source) {
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	copied, more, err := s.collect(runCtx, a, false)
	status, detail := "ok", ""
	if err != nil {
		status = "error"
		detail = s.safeError(a, err)
		s.log.Warn("mail collection failed", "source_id", a.ID, "user_id", a.UserID, "detail", detail)
	}
	next := time.Now().Add(time.Duration(a.Interval) * time.Minute).Unix()
	if more && err == nil {
		next = time.Now().Add(time.Minute).Unix()
	}
	// Release on shutdown too; the lease still recovers if the process was killed.
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	_, finishErr := s.db.ExecContext(finishCtx, s.query(`UPDATE mail_sources SET lease_until=0,lease_token='',last_run=?,last_status=?,last_error=?,next_run=?,copied=copied+? WHERE id=? AND lease_token=?`), time.Now().Unix(), status, detail, next, copied, a.ID, a.LeaseToken)
	if finishErr != nil {
		s.log.Warn("mail collection state failed", "source_id", a.ID, "error", finishErr)
	}
	u, e := s.store.GetUserByID(finishCtx, a.UserID)
	if e == nil {
		s.store.InsertMailLog(finishCtx, &storage.MailLogEntry{TenantID: u.TenantID, Domain: storage.DomainOfEmail(u.Email), Event: "collected", Direction: "inbound", Peer: a.Host, MailFrom: a.Username, RcptTo: u.Email, Detail: fmt.Sprintf("source_id=%s protocol=%s copied=%d status=%s %s", a.ID, a.Protocol, copied, status, detail)})
	}
}
func (s *Service) safeError(a *Source, err error) string {
	v := err.Error()
	if p, e := s.password(a); e == nil && p != "" {
		v = strings.ReplaceAll(v, p, "[redacted]")
	}
	v = strings.Join(strings.Fields(v), " ")
	if len(v) > 500 {
		v = v[:500]
	}
	return v
}
func (s *Service) Test(ctx context.Context, user, id string, in Settings) error {
	select {
	case s.testSlots <- struct{}{}:
		defer func() { <-s.testSlots }()
	default:
		return ErrBusy
	}
	if id != "" {
		if _, err := s.Get(ctx, user, id); err != nil {
			return err
		}
	}

	if err := validate(&in); err != nil {
		return err
	}
	a := &Source{ID: storage.NewID(), UserID: user, Protocol: in.Protocol, Host: in.Host, Port: in.Port, Security: in.Security, Username: in.Username, SourceFolder: in.SourceFolder}
	var err error
	if in.Password == "" && id != "" {
		old, e := s.Get(ctx, user, id)
		if e != nil {
			return e
		}
		in.Password, e = s.password(old)
		if e != nil {
			return errors.New("stored credentials unavailable")
		}
	}
	if in.Password == "" {
		return errors.New("password is required")
	}
	a.PasswordCiphertext, err = s.seal(user, a.ID, in.Password)
	if err != nil {
		return err
	}
	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, _, err = s.collect(testCtx, a, true)
	if err != nil {
		return errors.New(s.safeError(a, err))
	}
	return nil
}
func (s *Service) remotePrefix(a *Source) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(a.Protocol+"\x00"+a.Host+fmt.Sprint(a.Port)+"\x00"+a.Username+"\x00"+a.SourceFolder)))
}
func (s *Service) seen(ctx context.Context, a *Source, key string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, s.query(`SELECT COUNT(*) FROM mail_source_receipts WHERE source_id=? AND remote_key=?`), a.ID, key).Scan(&n)
	return n != 0, err
}
func (s *Service) deliver(ctx context.Context, a *Source, key string, raw []byte, flags string, date time.Time) (bool, error) {
	s.deliveryMu.Lock()
	defer s.deliveryMu.Unlock()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	u, err := s.store.GetUserByID(ctx, a.UserID)
	if err != nil || !u.Enabled {
		return false, errors.New("local account unavailable")
	}
	if s.Writers != nil {
		if err = s.Writers.AllowWrite(ctx, u.ID); err != nil {
			return false, err
		}
	}
	if err = storage.EnsureQuota(ctx, s.store, u, int64(len(raw))); err != nil {
		return false, err
	}
	// Use the authenticated local owner, never an identity from the external message.
	root, err := s.ms.EnsureFolder(u.Email, a.TargetFolder)
	if err != nil {
		return false, err
	}
	box, err := s.store.EnsureMailbox(ctx, u.ID, a.TargetFolder, root)
	if err != nil {
		return false, err
	}
	path, size, err := s.ms.Deliver(u.Email, a.TargetFolder, raw)
	if err != nil {
		return false, err
	}
	msg := &storage.Message{MailboxID: box.ID, FilePath: path, Size: size, Flags: flags, InternalDate: date, Archived: strings.EqualFold(a.TargetFolder, "Archive")}
	mailsearch.ApplyHeaders(msg, raw)
	inserted, err := s.store.InsertCollectedMessage(ctx, msg, a.ID, key)
	if err != nil {
		s.ms.Delete(path)
		if errors.Is(err, storage.ErrCollectedDuplicate) {
			return false, nil
		}
		return false, err
	}
	if err = mailsearch.Index(ctx, s.store, inserted.ID, raw); err != nil {
		s.log.Warn("collected mail search index failed", "message_id", inserted.ID, "error", err)
	}
	if s.Notifier != nil {
		s.Notifier.Notify(u.Email, a.TargetFolder)
	}
	return true, nil
}
