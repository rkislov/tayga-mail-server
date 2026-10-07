package imapserver

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	imapserv "github.com/emersion/go-imap/server"
	"github.com/emersion/go-sasl"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

const Delimiter = "/"

// Server wraps go-imap listeners.
type Server struct {
	cfg       *config.Config
	log       *slog.Logger
	store     storage.Driver
	authn     *auth.Layer
	mailstore *mailstore.Store
	servers   []*imapserv.Server
}

func New(cfg *config.Config, log *slog.Logger, store storage.Driver, authn *auth.Layer, ms *mailstore.Store) *Server {
	return &Server{cfg: cfg, log: log, store: store, authn: authn, mailstore: ms}
}

func (s *Server) Start(ctx context.Context) error {
	tlsCfg, err := s.loadTLS()
	if err != nil {
		return err
	}

	be := &Backend{log: s.log, store: s.store, authn: s.authn, ms: s.mailstore}

	type spec struct {
		addr     string
		name     string
		implicit bool
	}
	var specs []spec
	if s.cfg.IMAP.Listen != "" {
		specs = append(specs, spec{addr: s.cfg.IMAP.Listen, name: "imap"})
	}
	if s.cfg.IMAP.IMAPS != "" {
		specs = append(specs, spec{addr: s.cfg.IMAP.IMAPS, name: "imaps", implicit: true})
	}

	for _, sp := range specs {
		srv := imapserv.New(be)
		srv.Addr = sp.addr
		srv.AllowInsecureAuth = tlsCfg == nil
		if tlsCfg != nil {
			srv.TLSConfig = tlsCfg
		}
		s.enableOAuth(srv, be)

		var ln net.Listener
		if sp.implicit {
			if tlsCfg == nil {
				s.log.Warn("imaps configured but TLS certs missing; skipping", "addr", sp.addr)
				continue
			}
			ln, err = tls.Listen("tcp", sp.addr, tlsCfg)
		} else {
			ln, err = net.Listen("tcp", sp.addr)
		}
		if err != nil {
			_ = s.Shutdown(context.Background())
			return err
		}
		s.servers = append(s.servers, srv)
		s.log.Info("imap listening", "name", sp.name, "addr", sp.addr)
		go func(name string, srv *imapserv.Server, ln net.Listener) {
			if err := srv.Serve(ln); err != nil {
				s.log.Debug("imap serve stopped", "name", name, "err", err)
			}
		}(sp.name, srv, ln)
	}

	go func() {
		<-ctx.Done()
		_ = s.Shutdown(context.Background())
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	var first error
	for _, srv := range s.servers {
		done := make(chan error, 1)
		go func(srv *imapserv.Server) { done <- srv.Close() }(srv)
		select {
		case err := <-done:
			if err != nil && first == nil {
				first = err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return first
}

func (s *Server) loadTLS() (*tls.Config, error) {
	if !s.cfg.TLSEnabled() {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, nil
}

// Backend implements backend.Backend.
type Backend struct {
	log   *slog.Logger
	store storage.Driver
	authn *auth.Layer
	ms    *mailstore.Store
}

func (s *Server) enableOAuth(srv *imapserv.Server, be *Backend) {
	srv.EnableAuth(sasl.OAuthBearer, func(conn imapserv.Conn) sasl.Server {
		return sasl.NewOAuthBearerServer(func(opts sasl.OAuthBearerOptions) *sasl.OAuthBearerError {
			u, err := be.authn.AuthenticateToken(context.Background(), opts.Username, opts.Token)
			if err != nil {
				return &sasl.OAuthBearerError{Status: "invalid_token", Schemes: "bearer"}
			}
			user, err := be.userFromStorage(u)
			if err != nil {
				return &sasl.OAuthBearerError{Status: "invalid_token", Schemes: "bearer"}
			}
			ctx := conn.Context()
			ctx.State = imap.AuthenticatedState
			ctx.User = user
			return nil
		})
	})
	srv.EnableAuth(auth.XOAuth2, func(conn imapserv.Conn) sasl.Server {
		return auth.NewXOAuth2Server(func(username, token string) error {
			u, err := be.authn.AuthenticateToken(context.Background(), username, token)
			if err != nil {
				return err
			}
			user, err := be.userFromStorage(u)
			if err != nil {
				return err
			}
			ctx := conn.Context()
			ctx.State = imap.AuthenticatedState
			ctx.User = user
			return nil
		})
	})
}

func (b *Backend) Login(_ *imap.ConnInfo, username, password string) (backend.User, error) {
	u, err := b.authn.Authenticate(context.Background(), username, password)
	if err != nil {
		return nil, backend.ErrInvalidCredentials
	}
	return b.userFromStorage(u)
}

func (b *Backend) userFromStorage(u *storage.User) (backend.User, error) {
	if _, err := b.ms.EnsureUser(u.Email); err != nil {
		return nil, err
	}
	root := b.ms.UserRoot(u.Email)
	if _, err := b.store.EnsureMailbox(context.Background(), u.ID, "INBOX", root); err != nil {
		return nil, err
	}
	return &User{backend: b, user: u}, nil
}

type User struct {
	backend *Backend
	user    *storage.User
}

func (u *User) Username() string { return u.user.Email }

func (u *User) ListMailboxes(subscribed bool) ([]backend.Mailbox, error) {
	_ = subscribed
	ctx := context.Background()
	mbs, err := u.backend.store.ListMailboxes(ctx, u.user.ID)
	if err != nil {
		return nil, err
	}
	out := make([]backend.Mailbox, 0, len(mbs))
	for _, mb := range mbs {
		out = append(out, &Mailbox{user: u, mb: mb})
	}
	return out, nil
}

func (u *User) GetMailbox(name string) (backend.Mailbox, error) {
	mb, err := u.backend.store.GetMailbox(context.Background(), u.user.ID, name)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, backend.ErrNoSuchMailbox
		}
		return nil, err
	}
	return &Mailbox{user: u, mb: mb}, nil
}

func (u *User) CreateMailbox(name string) error {
	name = strings.TrimSuffix(name, Delimiter)
	if name == "" {
		return backend.ErrMailboxAlreadyExists
	}
	ctx := context.Background()
	if _, err := u.backend.store.GetMailbox(ctx, u.user.ID, name); err == nil {
		return backend.ErrMailboxAlreadyExists
	}
	path, err := u.backend.ms.EnsureFolder(u.user.Email, name)
	if err != nil {
		return err
	}
	_, err = u.backend.store.CreateMailbox(ctx, u.user.ID, name, path)
	return err
}

func (u *User) DeleteMailbox(name string) error {
	if strings.EqualFold(name, "INBOX") {
		return errors.New("cannot delete INBOX")
	}
	ctx := context.Background()
	mb, err := u.backend.store.GetMailbox(ctx, u.user.ID, name)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return backend.ErrNoSuchMailbox
		}
		return err
	}
	msgs, err := u.backend.store.ListMessages(ctx, mb.ID)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		_ = u.backend.ms.Delete(m.FilePath)
	}
	if err := u.backend.store.DeleteMailbox(ctx, u.user.ID, name); err != nil {
		return err
	}
	return u.backend.ms.RemoveFolder(u.user.Email, name)
}

func (u *User) RenameMailbox(existingName, newName string) error {
	newName = strings.TrimSuffix(newName, Delimiter)
	ctx := context.Background()
	if _, err := u.backend.store.GetMailbox(ctx, u.user.ID, existingName); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return backend.ErrNoSuchMailbox
		}
		return err
	}
	if _, err := u.backend.store.GetMailbox(ctx, u.user.ID, newName); err == nil {
		return backend.ErrMailboxAlreadyExists
	}
	if err := u.backend.ms.RenameFolder(u.user.Email, existingName, newName); err != nil {
		// INBOX special-case not fully implemented; surface error
		return err
	}
	path, err := u.backend.ms.EnsureFolder(u.user.Email, newName)
	if err != nil {
		return err
	}
	return u.backend.store.RenameMailbox(ctx, u.user.ID, existingName, newName, path)
}

func (u *User) Logout() error { return nil }

type Mailbox struct {
	user *User
	mb   *storage.Mailbox
}

func (m *Mailbox) Name() string { return m.mb.Name }

func (m *Mailbox) Info() (*imap.MailboxInfo, error) {
	return &imap.MailboxInfo{Delimiter: Delimiter, Name: m.mb.Name}, nil
}

func (m *Mailbox) reload() error {
	mb, err := m.user.backend.store.GetMailbox(context.Background(), m.user.user.ID, m.mb.Name)
	if err != nil {
		return err
	}
	m.mb = mb
	return nil
}

func (m *Mailbox) messages() ([]*storage.Message, error) {
	return m.user.backend.store.ListMessages(context.Background(), m.mb.ID)
}

func (m *Mailbox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	if err := m.reload(); err != nil {
		return nil, err
	}
	msgs, err := m.messages()
	if err != nil {
		return nil, err
	}
	status := imap.NewMailboxStatus(m.mb.Name, items)
	status.Flags = []string{imap.SeenFlag, imap.AnsweredFlag, imap.FlaggedFlag, imap.DeletedFlag, imap.DraftFlag}
	status.PermanentFlags = []string{"\\*"}
	status.UnseenSeqNum = unseenSeq(msgs)

	flagSet := map[string]struct{}{}
	var unseen, recent uint32
	for _, msg := range msgs {
		for _, f := range storage.ParseFlags(msg.Flags) {
			flagSet[f] = struct{}{}
		}
		if !storage.HasFlag(msg.Flags, imap.SeenFlag) {
			unseen++
		}
		if storage.MessageIsRecent(msg) {
			recent++
		}
	}
	var flags []string
	for f := range flagSet {
		flags = append(flags, f)
	}
	if len(flags) > 0 {
		status.Flags = flags
	}

	for _, name := range items {
		switch name {
		case imap.StatusMessages:
			status.Messages = uint32(len(msgs))
		case imap.StatusUidNext:
			status.UidNext = uint32(m.mb.UIDNext)
		case imap.StatusUidValidity:
			status.UidValidity = uint32(m.mb.UIDValidity)
		case imap.StatusRecent:
			status.Recent = recent
		case imap.StatusUnseen:
			status.Unseen = unseen
		}
	}
	return status, nil
}

func unseenSeq(msgs []*storage.Message) uint32 {
	for i, msg := range msgs {
		if !storage.HasFlag(msg.Flags, imap.SeenFlag) {
			return uint32(i + 1)
		}
	}
	return 0
}

func (m *Mailbox) SetSubscribed(bool) error { return nil }
func (m *Mailbox) Check() error             { return nil }

func (m *Mailbox) ListMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	defer close(ch)
	msgs, err := m.messages()
	if err != nil {
		return err
	}
	for i, msg := range msgs {
		seqNum := uint32(i + 1)
		id := seqNum
		if uid {
			id = uint32(msg.UID)
		}
		if !seqset.Contains(id) {
			continue
		}
		fetched, err := fetchMessage(m.user.backend.ms, msg, seqNum, items)
		if err != nil {
			m.user.backend.log.Warn("imap fetch failed", "uid", msg.UID, "err", err)
			continue
		}
		ch <- fetched
	}
	return nil
}

func (m *Mailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	msgs, err := m.messages()
	if err != nil {
		return nil, err
	}
	var ids []uint32
	for i, msg := range msgs {
		seqNum := uint32(i + 1)
		ok, err := matchMessage(m.user.backend.ms, msg, seqNum, criteria)
		if err != nil || !ok {
			continue
		}
		if uid {
			ids = append(ids, uint32(msg.UID))
		} else {
			ids = append(ids, seqNum)
		}
	}
	return ids, nil
}

func (m *Mailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if date.IsZero() {
		date = time.Now().UTC()
	}
	ctx := context.Background()
	if err := storage.EnsureQuota(ctx, m.user.backend.store, m.user.user, int64(len(data))); err != nil {
		return err
	}
	rel, size, err := m.user.backend.ms.Deliver(m.user.user.Email, m.mb.Name, data)
	if err != nil {
		return err
	}
	flagStr := storage.NormalizeFlags(flags)
	_, err = m.user.backend.store.InsertMessage(ctx, &storage.Message{
		MailboxID:    m.mb.ID,
		Size:         size,
		Flags:        flagStr,
		InternalDate: date,
		FilePath:     rel,
	})
	_ = m.reload()
	return err
}

func (m *Mailbox) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, op imap.FlagsOp, flags []string) error {
	msgs, err := m.messages()
	if err != nil {
		return err
	}
	ctx := context.Background()
	for i, msg := range msgs {
		id := uint32(i + 1)
		if uid {
			id = uint32(msg.UID)
		}
		if !seqset.Contains(id) {
			continue
		}
		cur := storage.ParseFlags(msg.Flags)
		newFlags := updateFlags(cur, op, flags)
		flagStr := storage.NormalizeFlags(newFlags)
		if err := m.user.backend.store.UpdateMessageFlags(ctx, msg.ID, flagStr); err != nil {
			return err
		}
		if newRel, err := m.user.backend.ms.MoveToCur(msg.FilePath, newFlags); err == nil && newRel != msg.FilePath {
			_ = m.user.backend.store.UpdateMessagePath(ctx, msg.ID, newRel)
		}
	}
	return nil
}

func updateFlags(current []string, op imap.FlagsOp, flags []string) []string {
	switch op {
	case imap.SetFlags:
		return append([]string(nil), flags...)
	case imap.AddFlags:
		set := map[string]string{}
		for _, f := range current {
			set[strings.ToLower(f)] = f
		}
		for _, f := range flags {
			set[strings.ToLower(f)] = f
		}
		out := make([]string, 0, len(set))
		for _, f := range set {
			out = append(out, f)
		}
		return out
	case imap.RemoveFlags:
		remove := map[string]struct{}{}
		for _, f := range flags {
			remove[strings.ToLower(f)] = struct{}{}
		}
		var out []string
		for _, f := range current {
			if _, ok := remove[strings.ToLower(f)]; !ok {
				out = append(out, f)
			}
		}
		return out
	default:
		return current
	}
}

func (m *Mailbox) CopyMessages(uid bool, seqset *imap.SeqSet, dest string) error {
	destMB, err := m.user.GetMailbox(dest)
	if err != nil {
		return err
	}
	dst := destMB.(*Mailbox)
	msgs, err := m.messages()
	if err != nil {
		return err
	}
	ctx := context.Background()
	for i, msg := range msgs {
		id := uint32(i + 1)
		if uid {
			id = uint32(msg.UID)
		}
		if !seqset.Contains(id) {
			continue
		}
		data, err := m.user.backend.ms.Read(msg.FilePath)
		if err != nil {
			return err
		}
		if err := storage.EnsureQuota(ctx, m.user.backend.store, m.user.user, int64(len(data))); err != nil {
			return err
		}
		rel, size, err := m.user.backend.ms.Deliver(m.user.user.Email, dst.mb.Name, data)
		if err != nil {
			return err
		}
		_, err = m.user.backend.store.InsertMessage(ctx, &storage.Message{
			MailboxID:    dst.mb.ID,
			Size:         size,
			Flags:        msg.Flags,
			InternalDate: msg.InternalDate,
			FilePath:     rel,
			MessageID:    msg.MessageID,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Mailbox) Expunge() error {
	ctx := context.Background()
	deleted, err := m.user.backend.store.ExpungeMailbox(ctx, m.mb.ID)
	if err != nil {
		return err
	}
	for _, msg := range deleted {
		_ = m.user.backend.ms.Delete(msg.FilePath)
	}
	return nil
}
