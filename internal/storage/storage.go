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

	InsertMessage(ctx context.Context, msg *Message) (*Message, error)
	GetMessageByID(ctx context.Context, id int64) (*Message, error)

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
