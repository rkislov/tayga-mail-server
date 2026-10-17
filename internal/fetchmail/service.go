// Package fetchmail collects external mail periodically without deleting source mail.
package fetchmail

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/ha"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/migrate"
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/storage"
)

var ErrBusy = errors.New("collection is running; try again after it finishes")
var ErrInvalid = errors.New("invalid mail source settings")
var ErrNotFound = errors.New("mail source not found")

type Source struct {
	ID                 string `json:"id"`
	UserID             string `json:"-"`
	Name               string `json:"name"`
	Protocol           string `json:"protocol"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Security           string `json:"security"`
	Username           string `json:"username"`
	PasswordCiphertext []byte `json:"-"`
	SourceFolder       string `json:"source_folder"`
	TargetFolder       string `json:"target_folder"`
	Interval           int    `json:"interval_minutes"`
	Enabled            bool   `json:"enabled"`
	NextRun            int64  `json:"next_run"`
	LastRun            int64  `json:"last_run"`
	Status             string `json:"status"`
	LastError          string `json:"last_error"`
	Copied             int64  `json:"copied"`
	LeaseUntil         int64  `json:"-"`
	LeaseToken         string `json:"-"`
	Created            int64  `json:"created_at"`
}
type Settings struct {
	Name         string `json:"name"`
	Protocol     string `json:"protocol"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Security     string `json:"security"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	SourceFolder string `json:"source_folder"`
	TargetFolder string `json:"target_folder"`
	Interval     int    `json:"interval_minutes"`
	Enabled      bool   `json:"enabled"`
}
type Service struct {
	store      *storage.Store
	db         *sql.DB
	ms         *mailstore.Store
	log        *slog.Logger
	key        []byte
	postgres   bool
	maxSize    int64
	localHosts []string
	Notifier   sieve.MailNotifier
	Writers    ha.WriterGate
	deliveryMu sync.Mutex
	testSlots  chan struct{}
	dial       func(context.Context, string, int) (net.Conn, error)
	tlsConfig  func(*Source) *tls.Config
}

func New(cfg *config.Config, log *slog.Logger, driver storage.Driver, ms *mailstore.Store) (*Service, error) {
	st, ok := driver.(*storage.Store)
	if !ok {
		return nil, nil
	}
	if log == nil {
		log = slog.Default()
	}
	key, err := migrate.ResolveKey(cfg.Server.SecretsKey, filepath.Dir(cfg.Mailstore.Root))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(append(key, []byte("tayga-mail-collection-v1")...))
	limit := cfg.SMTP.MaxSize
	if limit <= 0 {
		limit = 25 << 20
	}
	s := &Service{store: st, db: st.DB(), ms: ms, log: log, key: sum[:], postgres: cfg.Storage.Driver == "postgres", maxSize: limit, testSlots: make(chan struct{}, 4)}
	s.localHosts = []string{cfg.Server.Hostname}
	if publicURL, e := url.Parse(cfg.HTTP.PublicURL); e == nil {
		s.localHosts = append(s.localHosts, publicURL.Hostname())
	}
	s.dial = dialPublic
	s.tlsConfig = tlsConfig
	return s, nil
}
func (s *Service) query(q string) string {
	if !s.postgres {
		return q
	}
	n := 0
	return replaceParams(q, &n)
}
func replaceParams(q string, n *int) string {
	var b strings.Builder
	for _, r := range q {
		if r == '?' {
			*n++
			fmt.Fprintf(&b, "$%d", *n)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

const columns = `id,user_id,name,protocol,host,port,security,username,password_ciphertext,source_folder,target_folder,interval_minutes,enabled,next_run,last_run,last_status,last_error,copied,lease_until,lease_token,created_at`

func scan(row interface{ Scan(...any) error }) (*Source, error) {
	var a Source
	var enabled int
	err := row.Scan(&a.ID, &a.UserID, &a.Name, &a.Protocol, &a.Host, &a.Port, &a.Security, &a.Username, &a.PasswordCiphertext, &a.SourceFolder, &a.TargetFolder, &a.Interval, &enabled, &a.NextRun, &a.LastRun, &a.Status, &a.LastError, &a.Copied, &a.LeaseUntil, &a.LeaseToken, &a.Created)
	a.Enabled = enabled != 0
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return &a, err
}
func (s *Service) List(ctx context.Context, user string) ([]Source, error) {
	rows, err := s.db.QueryContext(ctx, s.query(`SELECT `+columns+` FROM mail_sources WHERE user_id=? ORDER BY created_at,id`), user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Source{}
	for rows.Next() {
		a, e := scan(rows)
		if e != nil {
			return nil, e
		}
		if a.LeaseUntil > time.Now().Unix() {
			a.Status = "running"
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}
func (s *Service) Get(ctx context.Context, user, id string) (*Source, error) {
	return scan(s.db.QueryRowContext(ctx, s.query(`SELECT `+columns+` FROM mail_sources WHERE user_id=? AND id=?`), user, id))
}
func validate(in *Settings) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(in.Host), "."))
	in.Username = strings.TrimSpace(in.Username)
	in.TargetFolder = strings.TrimSpace(in.TargetFolder)
	if strings.EqualFold(in.TargetFolder, "inbox") {
		in.TargetFolder = "INBOX"
	}
	if in.SourceFolder == "" {
		in.SourceFolder = "INBOX"
	}
	if in.Name == "" {
		in.Name = in.Username
	}
	if in.Protocol != "imap" && in.Protocol != "pop3" {
		return ErrInvalid
	}
	if in.Security != "tls" && in.Security != "starttls" && in.Security != "plain" {
		return ErrInvalid
	}
	if in.Port < 1 || in.Port > 65535 || in.Interval < 5 || in.Interval > 1440 || in.Host == "" || in.Username == "" || in.TargetFolder == "" {
		return ErrInvalid
	}
	for _, value := range []string{in.Name, in.Host, in.Username, in.Password, in.SourceFolder, in.TargetFolder} {
		if len(value) > 1024 || strings.ContainsFunc(value, unicode.IsControl) {
			return ErrInvalid
		}
	}
	if len(in.TargetFolder) > 200 {
		return ErrInvalid
	}
	if strings.ContainsAny(in.Host, "/\\@ ") || strings.ContainsAny(in.TargetFolder, "/\\") || in.TargetFolder == "." || in.TargetFolder == ".." || strings.HasPrefix(in.TargetFolder, ".") {
		return ErrInvalid
	}
	return nil
}
func (s *Service) seal(user, id, password string) ([]byte, error) {
	block, e := aes.NewCipher(s.key)
	if e != nil {
		return nil, e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	n := make([]byte, g.NonceSize())
	if _, e = rand.Read(n); e != nil {
		return nil, e
	}
	return g.Seal(n, n, []byte(password), []byte(user+"/"+id)), nil
}
func (s *Service) password(a *Source) (string, error) {
	block, e := aes.NewCipher(s.key)
	if e != nil {
		return "", e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return "", e
	}
	if len(a.PasswordCiphertext) < g.NonceSize() {
		return "", errors.New("stored credentials unavailable")
	}
	p, e := g.Open(nil, a.PasswordCiphertext[:g.NonceSize()], a.PasswordCiphertext[g.NonceSize():], []byte(a.UserID+"/"+a.ID))
	return string(p), e
}
func (s *Service) Save(ctx context.Context, user, id string, in Settings) (*Source, error) {
	if err := validate(&in); err != nil {
		return nil, err
	}
	var old *Source
	var err error
	if id != "" {
		old, err = s.Get(ctx, user, id)
		if err != nil {
			return nil, err
		}
		if old.LeaseUntil > time.Now().Unix() {
			return nil, ErrBusy
		}
	} else {
		id = storage.NewID()
	}
	if in.Password == "" && old == nil {
		return nil, errors.New("password is required")
	}
	var secret []byte
	if in.Password != "" {
		secret, err = s.seal(user, id, in.Password)
		if err != nil {
			return nil, err
		}
	} else {
		secret = old.PasswordCiphertext
	}
	enabled := 0
	if in.Enabled {
		enabled = 1
	}
	now := time.Now().Unix()
	if old == nil {
		result, e := s.db.ExecContext(ctx, s.query(`INSERT INTO mail_sources (`+columns+`) SELECT ?,?,?,?,?,?,?,?,?,?,?,?,?,0,0,'pending','',0,0,'',? WHERE (SELECT COUNT(*) FROM mail_sources WHERE user_id=?)<5`), id, user, in.Name, in.Protocol, in.Host, in.Port, in.Security, in.Username, secret, in.SourceFolder, in.TargetFolder, in.Interval, enabled, now, user)
		if e != nil {
			return nil, e
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return nil, errors.New("maximum five external accounts")
		}
	} else {
		result, e := s.db.ExecContext(ctx, s.query(`UPDATE mail_sources SET name=?,protocol=?,host=?,port=?,security=?,username=?,password_ciphertext=?,source_folder=?,target_folder=?,interval_minutes=?,enabled=?,next_run=0,last_status='pending',last_error='' WHERE id=? AND user_id=? AND lease_until<=?`), in.Name, in.Protocol, in.Host, in.Port, in.Security, in.Username, secret, in.SourceFolder, in.TargetFolder, in.Interval, enabled, id, user, now)
		if e != nil {
			return nil, e
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return nil, ErrBusy
		}
	}
	return s.Get(ctx, user, id)
}
func (s *Service) Delete(ctx context.Context, user, id string) error {
	if _, err := s.Get(ctx, user, id); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, s.query(`DELETE FROM mail_sources WHERE id=? AND user_id=? AND lease_until<=?`), id, user, time.Now().Unix())
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrBusy
	}
	return nil
}
func (s *Service) RunNow(ctx context.Context, user, id string) error {
	a, err := s.Get(ctx, user, id)
	if err != nil {
		return err
	}
	if !a.Enabled {
		return errors.New("enable the account before collecting")
	}
	result, err := s.db.ExecContext(ctx, s.query(`UPDATE mail_sources SET next_run=0 WHERE id=? AND user_id=? AND lease_until<=?`), id, user, time.Now().Unix())
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrBusy
	}
	return nil
}
