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
	UpdateDomainMigration(ctx context.Context, domainID, migrationEnabled string) error

	CreateUser(ctx context.Context, u *User) (*User, error)
	UpdateUserRoles(ctx context.Context, userID, roles string) error
	UpdateUserServiceClass(ctx context.Context, userID, serviceClassID string) error
	UpdateUserMigration(ctx context.Context, userID, migrationEnabled string) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	UpdateUserPassword(ctx context.Context, userID, passwordHash string) error
	UpdateUserProfile(ctx context.Context, userID, displayName string) error
	UpdateUserQuota(ctx context.Context, userID string, quotaBytes int64) error
	UpdateUserEnabled(ctx context.Context, userID string, enabled bool) error
	DeleteUser(ctx context.Context, userID string) error
	ListUsersByTenant(ctx context.Context, tenantID string) ([]*User, error)
	ListUsersByDomain(ctx context.Context, domainID string) ([]*User, error)
	CreateCalendarResource(ctx context.Context, res *CalendarResource) (*CalendarResource, error)
	UpdateCalendarResource(ctx context.Context, res *CalendarResource) error
	DeleteCalendarResource(ctx context.Context, id string) error
	GetCalendarResource(ctx context.Context, id string) (*CalendarResource, error)
	GetCalendarResourceByEmail(ctx context.Context, email string) (*CalendarResource, error)
	GetCalendarResourceByUserID(ctx context.Context, userID string) (*CalendarResource, error)
	ListCalendarResourcesByDomain(ctx context.Context, domainID string, enabledOnly bool) ([]*CalendarResource, error)
	ListCalendarResourcesByTenant(ctx context.Context, tenantID string, enabledOnly bool) ([]*CalendarResource, error)
	CreateCalendarAttachment(ctx context.Context, a *CalendarAttachment) (*CalendarAttachment, error)
	GetCalendarAttachment(ctx context.Context, id string) (*CalendarAttachment, error)
	ListCalendarAttachments(ctx context.Context, calendarObjectID string) ([]*CalendarAttachment, error)
	DeleteCalendarAttachment(ctx context.Context, id string) error
	SumMailboxBytes(ctx context.Context, userID string) (int64, error)
	MessageExistsByMessageID(ctx context.Context, mailboxID, messageID string, size int64) (bool, error)
	CalendarObjectExistsByUID(ctx context.Context, calendarID, uid string) (bool, error)
	AddressObjectExistsByUID(ctx context.Context, bookID, uid string) (bool, error)

	SetDomainAdminDomains(ctx context.Context, userID string, domainIDs []string) error
	ListDomainAdminDomains(ctx context.Context, userID string) ([]string, error)
	UserAdministersDomain(ctx context.Context, userID, domainID string) (bool, error)

	CreateFileShare(ctx context.Context, sh *FileShare) (*FileShare, error)
	GetFileShareByToken(ctx context.Context, token string) (*FileShare, error)
	GetFileShareByID(ctx context.Context, id string) (*FileShare, error)
	ListFileShares(ctx context.Context, userID string) ([]*FileShare, error)
	DeleteFileShare(ctx context.Context, userID, id string) error
	IncrementFileShareDownload(ctx context.Context, id string) error

	UpsertMailboxDelegate(ctx context.Context, d *MailboxDelegate) error
	ListMailboxDelegates(ctx context.Context, ownerID string) ([]*MailboxDelegate, error)
	ListDelegationsFor(ctx context.Context, delegateID string) ([]*MailboxDelegate, error)
	GetMailboxDelegate(ctx context.Context, ownerID, delegateID string) (*MailboxDelegate, error)
	DeleteMailboxDelegate(ctx context.Context, ownerID, delegateID string) error

	SetMailboxACL(ctx context.Context, mailboxID, granteeUserID, rights string) error
	DeleteMailboxACL(ctx context.Context, mailboxID, granteeUserID string) error
	ListMailboxACL(ctx context.Context, mailboxID string) ([]*MailboxACLEntry, error)
	MailboxRightsForUser(ctx context.Context, mailboxID, userID string) (string, error)
	ListSharedMailboxes(ctx context.Context, userID string) ([]*Mailbox, error)

	SetCalendarACL(ctx context.Context, calendarID, granteeUserID, rights string) error
	DeleteCalendarACL(ctx context.Context, calendarID, granteeUserID string) error
	ListCalendarACL(ctx context.Context, calendarID string) ([]*CalendarACLEntry, error)
	CalendarRightsForUser(ctx context.Context, calendarID, userID string) (string, error)
	ListSharedCalendars(ctx context.Context, userID string) ([]*Calendar, error)

	CreateServiceClass(ctx context.Context, sc *ServiceClass) (*ServiceClass, error)
	UpdateServiceClass(ctx context.Context, sc *ServiceClass) error
	DeleteServiceClass(ctx context.Context, tenantID, id string) error
	GetServiceClass(ctx context.Context, id string) (*ServiceClass, error)
	ListServiceClasses(ctx context.Context, tenantID string) ([]*ServiceClass, error)

	CreateAlias(ctx context.Context, domainID, userID, localPart string) (*Alias, error)
	ResolveRecipient(ctx context.Context, email string) (*User, error)

	EnsureMailbox(ctx context.Context, userID, name, path string) (*Mailbox, error)
	GetMailbox(ctx context.Context, userID, name string) (*Mailbox, error)
	GetMailboxByID(ctx context.Context, id string) (*Mailbox, error)
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
	UpdateMessageArchiveMeta(ctx context.Context, messageID, filePath string, size int64, archived bool, subject, fromAddr, toAddr, dateHdr string) error
	UpdateMessageHeaders(ctx context.Context, messageID, subject, fromAddr, toAddr, dateHdr string) error
	UpsertMessageSearch(ctx context.Context, messageID, subject, fromAddr, toAddr, body string) error
	DeleteMessageSearch(ctx context.Context, messageID string) error
	SearchMessages(ctx context.Context, userID, mailboxID, fromFilter, toFilter, subjectFilter, ftsQuery string, limit int, options ...MessageSearchOptions) ([]SearchHit, error)
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
	UpdateCalendar(ctx context.Context, c *Calendar) error
	GetPublicCalendar(ctx context.Context, token string) (*Calendar, error)
	ListCalendars(ctx context.Context, userID string) ([]*Calendar, error)
	GetCalendarByName(ctx context.Context, userID, name string) (*Calendar, error)
	GetCalendarByID(ctx context.Context, userID, id string) (*Calendar, error)
	DeleteCalendar(ctx context.Context, userID, name string) error
	ListCalendarObjects(ctx context.Context, calendarID string) ([]*CalendarObject, error)
	GetCalendarObject(ctx context.Context, calendarID, hrefName string) (*CalendarObject, error)
	GetCalendarObjectByID(ctx context.Context, id string) (*CalendarObject, error)
	UpsertCalendarObject(ctx context.Context, o *CalendarObject) (*CalendarObject, error)
	DeleteCalendarObject(ctx context.Context, calendarID, hrefName string) error
	UpsertCalendarInvite(ctx context.Context, inv *CalendarInvite) (*CalendarInvite, error)
	GetCalendarInvite(ctx context.Context, id string) (*CalendarInvite, error)
	ListCalendarInvitesForUser(ctx context.Context, userID string, pendingOnly bool) ([]*CalendarInvite, error)
	ListCalendarInvitesByEventUID(ctx context.Context, eventUID string) ([]*CalendarInvite, error)
	UpdateCalendarInvitePartStat(ctx context.Context, id, partstat string, proposedStart, proposedEnd *time.Time) error
	FreeBusyForUser(ctx context.Context, userID string, from, to time.Time) ([]BusyInterval, error)

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

	EnsureNoteDefaults(ctx context.Context, userID string) error
	EnsureNoteFolder(ctx context.Context, userID, name, displayName string) (*NoteFolder, error)
	CreateNoteFolder(ctx context.Context, f *NoteFolder) (*NoteFolder, error)
	ListNoteFolders(ctx context.Context, userID string) ([]*NoteFolder, error)
	ListNoteFoldersForUser(ctx context.Context, userID string) ([]*NoteFolder, error)
	ListSharedNoteFolders(ctx context.Context, userID string) ([]*NoteFolder, error)
	GetNoteFolderByName(ctx context.Context, userID, name string) (*NoteFolder, error)
	GetNoteFolderByID(ctx context.Context, userID, id string) (*NoteFolder, error)
	GetNoteFolder(ctx context.Context, id string) (*NoteFolder, error)
	UpdateNoteFolder(ctx context.Context, f *NoteFolder) error
	DeleteNoteFolder(ctx context.Context, userID, id string) error
	CreateNoteItem(ctx context.Context, n *NoteItem) (*NoteItem, error)
	UpdateNoteItem(ctx context.Context, n *NoteItem) error
	GetNoteItem(ctx context.Context, userID, id string) (*NoteItem, error)
	GetNoteItemByID(ctx context.Context, id string) (*NoteItem, error)
	ListNoteItems(ctx context.Context, userID, folderID string, includeArchived bool) ([]*NoteItem, error)
	ListNoteItemsInFolder(ctx context.Context, folderID string, includeArchived bool) ([]*NoteItem, error)
	ListSharedNoteItems(ctx context.Context, userID string) ([]*NoteItem, error)
	DeleteNoteItem(ctx context.Context, userID, id string) error
	DeleteNoteItemByID(ctx context.Context, id string) error
	CreateNoteAttachment(ctx context.Context, a *NoteAttachment) (*NoteAttachment, error)
	GetNoteAttachment(ctx context.Context, id string) (*NoteAttachment, error)
	ListNoteAttachments(ctx context.Context, noteID string) ([]*NoteAttachment, error)
	DeleteNoteAttachment(ctx context.Context, id string) error
	SetNoteFolderACL(ctx context.Context, folderID, granteeUserID, rights string) error
	DeleteNoteFolderACL(ctx context.Context, folderID, granteeUserID string) error
	ListNoteFolderACL(ctx context.Context, folderID string) ([]*NoteFolderACLEntry, error)
	SetNoteACL(ctx context.Context, noteID, granteeUserID, rights string) error
	DeleteNoteACL(ctx context.Context, noteID, granteeUserID string) error
	ListNoteACL(ctx context.Context, noteID string) ([]*NoteACLEntry, error)
	NoteFolderRightsForUser(ctx context.Context, folderID, userID string) (string, error)
	NoteRightsForUser(ctx context.Context, noteID, userID string) (string, error)

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
	ListOutbound(ctx context.Context, limit int) ([]*OutboundItem, error)
	GetOutbound(ctx context.Context, id string) (*OutboundItem, error)
	RescheduleOutbound(ctx context.Context, id string, attempts int, nextAttempt time.Time, lastError string) error
	DeleteOutbound(ctx context.Context, id string) error
	CountOutbound(ctx context.Context) (int, error)

	UpsertDMARCAgg(ctx context.Context, row *DMARCAggRow) error
	ListDMARCAggByDay(ctx context.Context, day string) ([]*DMARCAggRow, error)
	DeleteDMARCAggByDay(ctx context.Context, day string) error
	ListDMARCAggDaysBefore(ctx context.Context, beforeDay string) ([]string, error)

	// GreylistTouch records/updates a triplet; allowed=false means defer (421).
	GreylistTouch(ctx context.Context, clientIP, envelopeFrom, rcpt string, delay time.Duration) (allowed bool, err error)
	DeleteExpiredGreylist(ctx context.Context, olderThan time.Time) (int64, error)

	GetSetting(ctx context.Context, key string) (value string, ok bool, err error)
	ListSettings(ctx context.Context) (map[string]string, error)
	PutSetting(ctx context.Context, key, value string) error

	// TryAcquireUserWriter grabs or renews a per-user maildir writer lease for nodeID.
	// acquired=false means another live node holds it (holder is that node_id).
	TryAcquireUserWriter(ctx context.Context, userID, nodeID string, ttl time.Duration) (acquired bool, holder string, err error)
	ReleaseUserWriter(ctx context.Context, userID, nodeID string) error

	ServerStats(ctx context.Context) (*ServerStats, error)
	TenantStats(ctx context.Context, tenantID string) (*TenantStats, error)

	InsertMailLog(ctx context.Context, e *MailLogEntry) error
	SearchMailLog(ctx context.Context, q MailLogQuery) ([]MailLogEntry, error)

	DB() *sql.DB
}

type Tenant struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type Domain struct {
	ID               string
	TenantID         string
	Name             string
	MigrationEnabled string // inherit | on | off
	CreatedAt        time.Time
}

type User struct {
	ID               string
	TenantID         string
	DomainID         string
	Email            string
	LocalPart        string
	DisplayName      string
	PasswordHash     string
	AuthSource       string // local | ldap | oidc
	QuotaBytes       int64
	Enabled          bool
	Roles            string // comma-separated: global_admin, domain_admin, …
	ServiceClassID   string
	MigrationEnabled string // inherit | on | off
	CreatedAt        time.Time
}

type DomainAdminBinding struct {
	UserID   string
	DomainID string
}

type FileShare struct {
	ID            string
	UserID        string
	Path          string
	Token         string
	ExpiresAt     *time.Time
	MaxDownloads  int
	DownloadCount int
	CreatedAt     time.Time
}

type MailboxDelegate struct {
	OwnerID         string
	DelegateID      string
	CanRead         bool
	CanSendAs       bool
	CanSendOnBehalf bool
}

type MailboxACLEntry struct {
	MailboxID     string
	GranteeUserID string
	Rights        string
}

type CalendarACLEntry struct {
	CalendarID    string
	GranteeUserID string
	Rights        string // read | write | freebusy
}

type ServiceClass struct {
	ID        string
	TenantID  string
	Name      string
	Config    string // JSON
	CreatedAt time.Time
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
	Subject      string
	FromAddr     string
	ToAddr       string
	DateHdr      string
	Archived     bool
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

// DMARCAggRow is one aggregated DMARC feedback row for a calendar day (UTC).
type DMARCAggRow struct {
	ID             string
	Domain         string // organizational / policy domain (header From)
	Day            string // YYYY-MM-DD UTC
	SourceIP       string
	EnvelopeDomain string
	HeaderFrom     string
	SPFResult      string // pass|fail|none
	DKIMResult     string // pass|fail|none
	Disposition    string // none|quarantine|reject
	Policy         string // published p=
	RUA            string // comma-separated mailto URIs
	Count          int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
