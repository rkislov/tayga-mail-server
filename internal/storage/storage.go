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
	ErrQuotaExceeded = errors.New("mailbox quota exceeded")
)

// Driver is the storage abstraction shared by protocol handlers.
type Driver interface {
	Ping(ctx context.Context) error
	Close() error
	Migrate(ctx context.Context) error

	CreateTenant(ctx context.Context, name string) (*Tenant, error)
	InsertTenant(ctx context.Context, t *Tenant) (*Tenant, error)
	GetTenantByName(ctx context.Context, name string) (*Tenant, error)
	GetTenantByID(ctx context.Context, id string) (*Tenant, error)
	ListTenants(ctx context.Context) ([]*Tenant, error)

	CreateDomain(ctx context.Context, tenantID, name string) (*Domain, error)
	InsertDomain(ctx context.Context, d *Domain) (*Domain, error)
	GetDomainByName(ctx context.Context, name string) (*Domain, error)
	GetDomainByID(ctx context.Context, id string) (*Domain, error)
	ListDomainsByTenant(ctx context.Context, tenantID string) ([]*Domain, error)
	CountUsersByDomain(ctx context.Context, domainID string) (int, error)
	DeleteDomain(ctx context.Context, tenantID, domainID string) error

	CreateUser(ctx context.Context, u *User) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	UpdateUserPassword(ctx context.Context, userID, passwordHash string) error
	UpdateUserProfile(ctx context.Context, userID, displayName string) error
	UpdateUserQuota(ctx context.Context, userID string, quotaBytes int64) error
	UpdateUserEnabled(ctx context.Context, userID string, enabled bool) error
	ListUsersByTenant(ctx context.Context, tenantID string) ([]*User, error)
	SumMailboxBytes(ctx context.Context, userID string) (int64, error)

	CreateAlias(ctx context.Context, domainID, userID, localPart string) (*Alias, error)
	ResolveRecipient(ctx context.Context, email string) (*User, error)

	EnsureMailbox(ctx context.Context, userID, name, path string) (*Mailbox, error)
	GetMailbox(ctx context.Context, userID, name string) (*Mailbox, error)
	ListMailboxes(ctx context.Context, userID string) ([]*Mailbox, error)
	CreateMailbox(ctx context.Context, userID, name, path string) (*Mailbox, error)
	DeleteMailbox(ctx context.Context, userID, name string) error
	RenameMailbox(ctx context.Context, userID, oldName, newName, newPath string) error

	InsertMessage(ctx context.Context, msg *Message) (*Message, error)
	GetMessageByID(ctx context.Context, id string) (*Message, error)
	GetMessageByUID(ctx context.Context, mailboxID string, uid int64) (*Message, error)
	ListMessages(ctx context.Context, mailboxID string) ([]*Message, error)
	UpdateMessageFlags(ctx context.Context, messageID, flags string) error
	UpdateMessagePath(ctx context.Context, messageID, filePath string) error
	MoveMessage(ctx context.Context, messageID, dstMailboxID string) (*Message, error)
	DeleteMessage(ctx context.Context, messageID string) error
	ExpungeMailbox(ctx context.Context, mailboxID string) ([]*Message, error)

	ListSieveScripts(ctx context.Context, userID string) ([]*SieveScript, error)
	GetSieveScript(ctx context.Context, userID, name string) (*SieveScript, error)
	GetActiveSieveScript(ctx context.Context, userID string) (*SieveScript, error)
	PutSieveScript(ctx context.Context, userID, name, script string) (*SieveScript, error)
	DeleteSieveScript(ctx context.Context, userID, name string) error
	SetActiveSieveScript(ctx context.Context, userID, name string) error

	GetUserMFA(ctx context.Context, userID string) (*UserMFA, error)
	UpsertUserMFA(ctx context.Context, m *UserMFA) error
	CreateOAuthToken(ctx context.Context, t *OAuthToken) (*OAuthToken, error)
	GetOAuthTokenByAccess(ctx context.Context, accessToken string) (*OAuthToken, error)
	GetOAuthTokenByRefresh(ctx context.Context, refreshToken string) (*OAuthToken, error)
	DeleteOAuthToken(ctx context.Context, id string) error
	DeleteOAuthTokensByUser(ctx context.Context, userID string) error
	CreateMFAChallenge(ctx context.Context, c *MFAChallenge) error
	GetMFAChallenge(ctx context.Context, token string) (*MFAChallenge, error)
	DeleteMFAChallenge(ctx context.Context, token string) error

	EnsureCalendar(ctx context.Context, userID, name, displayName string) (*Calendar, error)
	CreateCalendar(ctx context.Context, c *Calendar) (*Calendar, error)
	ListCalendars(ctx context.Context, userID string) ([]*Calendar, error)
	GetCalendarByName(ctx context.Context, userID, name string) (*Calendar, error)
	GetCalendarByID(ctx context.Context, userID, id string) (*Calendar, error)
	DeleteCalendar(ctx context.Context, userID, name string) error
	ListCalendarObjects(ctx context.Context, calendarID string) ([]*CalendarObject, error)
	GetCalendarObject(ctx context.Context, calendarID, hrefName string) (*CalendarObject, error)
	GetCalendarObjectByID(ctx context.Context, id string) (*CalendarObject, error)
	UpsertCalendarObject(ctx context.Context, o *CalendarObject) (*CalendarObject, error)
	DeleteCalendarObject(ctx context.Context, calendarID, hrefName string) error

	EnsureAddressBook(ctx context.Context, userID, name, displayName string) (*AddressBook, error)
	CreateAddressBook(ctx context.Context, ab *AddressBook) (*AddressBook, error)
	ListAddressBooks(ctx context.Context, userID string) ([]*AddressBook, error)
	GetAddressBookByName(ctx context.Context, userID, name string) (*AddressBook, error)
	GetAddressBookByID(ctx context.Context, userID, id string) (*AddressBook, error)
	DeleteAddressBook(ctx context.Context, userID, name string) error
	ListAddressObjects(ctx context.Context, addressBookID string) ([]*AddressObject, error)
	GetAddressObject(ctx context.Context, addressBookID, hrefName string) (*AddressObject, error)
	GetAddressObjectByID(ctx context.Context, id string) (*AddressObject, error)
	UpsertAddressObject(ctx context.Context, o *AddressObject) (*AddressObject, error)
	DeleteAddressObject(ctx context.Context, addressBookID, hrefName string) error
	EnsureDAVDefaults(ctx context.Context, userID string) error

	EnsureFlowSyncDevice(ctx context.Context, userID, deviceID, deviceType string) (*FlowSyncDevice, error)
	SetFlowSyncPolicyKey(ctx context.Context, deviceRowID, policyKey string) error
	GetFlowSyncSyncKey(ctx context.Context, deviceRowID, collectionID string) (string, error)
	SetFlowSyncSyncKey(ctx context.Context, deviceRowID, collectionID, syncKey string) error

	CreateWebAuthnCredential(ctx context.Context, c *WebAuthnCredential) (*WebAuthnCredential, error)
	ListWebAuthnCredentials(ctx context.Context, userID string) ([]*WebAuthnCredential, error)
	GetWebAuthnCredential(ctx context.Context, id string) (*WebAuthnCredential, error)
	GetWebAuthnCredentialByCredID(ctx context.Context, credentialID string) (*WebAuthnCredential, error)
	UpdateWebAuthnCredential(ctx context.Context, c *WebAuthnCredential) error
	DeleteWebAuthnCredential(ctx context.Context, userID, id string) error
	CountWebAuthnCredentials(ctx context.Context, userID string) (int, error)
	CreateWebAuthnSession(ctx context.Context, sess *WebAuthnSession) (*WebAuthnSession, error)
	GetWebAuthnSession(ctx context.Context, id string) (*WebAuthnSession, error)
	DeleteWebAuthnSession(ctx context.Context, id string) error

	EnqueueOutbound(ctx context.Context, item *OutboundItem) (*OutboundItem, error)
	ClaimOutboundDue(ctx context.Context, limit int) ([]*OutboundItem, error)
	RescheduleOutbound(ctx context.Context, id string, attempts int, nextAttempt time.Time, lastError string) error
	DeleteOutbound(ctx context.Context, id string) error
	CountOutbound(ctx context.Context) (int, error)

	ServerStats(ctx context.Context) (*ServerStats, error)
	TenantStats(ctx context.Context, tenantID string) (*TenantStats, error)

	DB() *sql.DB
}

type Tenant struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type Domain struct {
	ID        string
	TenantID  string
	Name      string
	CreatedAt time.Time
}

type User struct {
	ID           string
	TenantID     string
	DomainID     string
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
	ID        string
	DomainID  string
	UserID    string
	LocalPart string
	CreatedAt time.Time
}

type Mailbox struct {
	ID          string
	UserID      string
	Name        string
	Path        string
	UIDNext     int64
	UIDValidity int64
	CreatedAt   time.Time
}

type Message struct {
	ID           string
	MailboxID    string
	UID          int64
	Size         int64
	Flags        string
	InternalDate time.Time
	FilePath     string
	MessageID    string
	CreatedAt    time.Time
}

type SieveScript struct {
	ID        string
	UserID    string
	Name      string
	Script    string
	Active    bool
	CreatedAt time.Time
}

// OutboundItem is one queued remote delivery (one recipient).
type OutboundItem struct {
	ID           string
	EnvelopeFrom string
	EnvelopeTo   string
	MessageID    string
	Data         []byte
	Attempts     int
	MaxAttempts  int
	NextAttempt  time.Time
	LastError    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
