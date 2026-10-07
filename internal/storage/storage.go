package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrUnauthorized  = errors.New("unauthorized")
)

// Driver is the storage abstraction shared by protocol handlers.
type Driver interface {
	Ping(ctx context.Context) error
	Close() error
	Migrate(ctx context.Context) error

	CreateTenant(ctx context.Context, name string) (*Tenant, error)
	GetTenantByName(ctx context.Context, name string) (*Tenant, error)

	CreateDomain(ctx context.Context, tenantID int64, name string) (*Domain, error)
	GetDomainByName(ctx context.Context, name string) (*Domain, error)

	CreateUser(ctx context.Context, u *User) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id int64) (*User, error)
	UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error

	CreateAlias(ctx context.Context, domainID, userID int64, localPart string) (*Alias, error)
	ResolveRecipient(ctx context.Context, email string) (*User, error)

	EnsureMailbox(ctx context.Context, userID int64, name, path string) (*Mailbox, error)
	GetMailbox(ctx context.Context, userID int64, name string) (*Mailbox, error)
	ListMailboxes(ctx context.Context, userID int64) ([]*Mailbox, error)
	CreateMailbox(ctx context.Context, userID int64, name, path string) (*Mailbox, error)
	DeleteMailbox(ctx context.Context, userID int64, name string) error
	RenameMailbox(ctx context.Context, userID int64, oldName, newName, newPath string) error

	InsertMessage(ctx context.Context, msg *Message) (*Message, error)
	GetMessageByID(ctx context.Context, id int64) (*Message, error)
	GetMessageByUID(ctx context.Context, mailboxID, uid int64) (*Message, error)
	ListMessages(ctx context.Context, mailboxID int64) ([]*Message, error)
	UpdateMessageFlags(ctx context.Context, messageID int64, flags string) error
	UpdateMessagePath(ctx context.Context, messageID int64, filePath string) error
	DeleteMessage(ctx context.Context, messageID int64) error
	ExpungeMailbox(ctx context.Context, mailboxID int64) ([]*Message, error) // returns deleted msgs (had \Deleted)

	ListSieveScripts(ctx context.Context, userID int64) ([]*SieveScript, error)
	GetSieveScript(ctx context.Context, userID int64, name string) (*SieveScript, error)
	GetActiveSieveScript(ctx context.Context, userID int64) (*SieveScript, error)
	PutSieveScript(ctx context.Context, userID int64, name, script string) (*SieveScript, error)
	DeleteSieveScript(ctx context.Context, userID int64, name string) error
	SetActiveSieveScript(ctx context.Context, userID int64, name string) error // empty name = deactivate all

	DB() *sql.DB
}

type Tenant struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

type Domain struct {
	ID        int64
	TenantID  int64
	Name      string
	CreatedAt time.Time
}

type User struct {
	ID           int64
	TenantID     int64
	DomainID     int64
	Email        string
	LocalPart    string
	DisplayName  string
	PasswordHash string
	AuthSource   string // local | ldap | oidc
	QuotaBytes   int64
	Enabled      bool
	CreatedAt    time.Time
}

type Alias struct {
	ID        int64
	DomainID  int64
	UserID    int64
	LocalPart string
	CreatedAt time.Time
}

type Mailbox struct {
	ID          int64
	UserID      int64
	Name        string
	Path        string
	UIDNext     int64
	UIDValidity int64
	CreatedAt   time.Time
}

type Message struct {
	ID           int64
	MailboxID    int64
	UID          int64
	Size         int64
	Flags        string
	InternalDate time.Time
	FilePath     string
	MessageID    string
	CreatedAt    time.Time
}

type SieveScript struct {
	ID        int64
	UserID    int64
	Name      string
	Script    string
	Active    bool
	CreatedAt time.Time
}
