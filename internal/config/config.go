package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level server configuration loaded from YAML.
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Storage     StorageConfig     `yaml:"storage"`
	Mailstore   MailstoreConfig   `yaml:"mailstore"`
	SMTP        SMTPConfig        `yaml:"smtp"`
	IMAP        IMAPConfig        `yaml:"imap"`
	POP3        POP3Config        `yaml:"pop3"`
	ManageSieve ManageSieveConfig `yaml:"managesieve"`
	LDAP        LDAPConfig        `yaml:"ldap"`
	OIDC        OIDCConfig        `yaml:"oidc"`
	MFA         MFAConfig         `yaml:"mfa"`
	FlowSync    FlowSyncConfig    `yaml:"flowsync"`
	TLS         TLSConfig         `yaml:"tls"`
	HTTP        HTTPConfig        `yaml:"http"`
	Seed        SeedConfig        `yaml:"seed"`
	HA          HAConfig          `yaml:"ha"`
	Scan        ScanConfig        `yaml:"scan"`
	Spam        SpamConfig        `yaml:"spam"`
	DKIMVerify  DKIMVerifyConfig  `yaml:"dkim_verify"`
	SPF         SPFConfig         `yaml:"spf"`
	DMARC       DMARCConfig       `yaml:"dmarc"`
	Log         LogConfig         `yaml:"log"`
}

// DKIMVerifyConfig checks inbound DKIM signatures (unauthenticated MX).
type DKIMVerifyConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Action           string `yaml:"action"`            // tag | reject
	RequireSignature bool   `yaml:"require_signature"` // reject/fail when no DKIM-Signature
	FailOpen         bool   `yaml:"fail_open"`         // temperror → deliver (tag temperror)
	AuthservID       string `yaml:"authserv_id"`       // Authentication-Results id (default server.hostname)
}

// SPFConfig checks inbound SPF for unauthenticated MX.
type SPFConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Action     string `yaml:"action"` // tag | reject
	FailOpen   bool   `yaml:"fail_open"`
	AuthservID string `yaml:"authserv_id"`
}

// DMARCConfig evaluates DMARC using SPF + DKIM alignment.
type DMARCConfig struct {
	Enabled    bool              `yaml:"enabled"`
	Action     string            `yaml:"action"` // tag | reject | follow (honor p=reject)
	FailOpen   bool              `yaml:"fail_open"`
	AuthservID string            `yaml:"authserv_id"`
	Report     DMARCReportConfig `yaml:"report"`
}

// DMARCReportConfig sends RFC 7489 aggregate (rua) reports.
type DMARCReportConfig struct {
	Enabled  bool          `yaml:"enabled"`
	OrgName  string        `yaml:"org_name"` // report_metadata/org_name
	Contact  string        `yaml:"contact"`  // report From / metadata email
	Interval time.Duration `yaml:"interval"` // flush cadence (default 24h)
}

// SpamConfig configures Rspamd (or similar) scoring for inbound SMTP.
type SpamConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Backend         string        `yaml:"backend"` // rspamd | none
	URL             string        `yaml:"url"`
	Password        string        `yaml:"password"`
	Timeout         time.Duration `yaml:"timeout"`
	FailOpen        bool          `yaml:"fail_open"`
	Folder          string        `yaml:"folder"` // Junk / Quarantine for quarantine action
	FollowRspamd    bool          `yaml:"follow_rspamd"`
	RejectAbove     float64       `yaml:"reject_above"`
	QuarantineAbove float64       `yaml:"quarantine_above"`
	TagAbove        float64       `yaml:"tag_above"`
}

// ScanConfig configures antivirus scanning of inbound SMTP messages.
type ScanConfig struct {
	Enabled          bool           `yaml:"enabled"`
	Backend          string         `yaml:"backend"` // clamav | exec | none
	Action           string         `yaml:"action"`  // reject | quarantine | tag
	QuarantineFolder string         `yaml:"quarantine_folder"`
	Timeout          time.Duration  `yaml:"timeout"`
	FailOpen         bool           `yaml:"fail_open"` // deliver on scanner errors
	ClamAV           ClamAVConfig   `yaml:"clamav"`
	Exec             ExecScanConfig `yaml:"exec"`
}

type ClamAVConfig struct {
	Address string `yaml:"address"` // host:port of clamd
}

type ExecScanConfig struct {
	Command []string `yaml:"command"` // e.g. [clamdscan, --fdpass, --no-summary, -]
}

// HAConfig enables optional active/standby or per-user sticky writer fencing.
type HAConfig struct {
	Mode      string        `yaml:"mode"`       // none | active_standby | sticky
	LeaseTTL  time.Duration `yaml:"lease_ttl"`  // active_standby re-check (default 5s)
	Fence     string        `yaml:"fence"`      // mx | writers (active_standby only; default mx)
	StickyTTL time.Duration `yaml:"sticky_ttl"` // sticky lease TTL (default 30s)
	NodeID    string        `yaml:"node_id"`    // sticky node identity (default hostname-uuid)
}

// FenceWriters reports whether HA should gate all maildir writers behind the cluster leader.
func (c HAConfig) FenceWriters() bool {
	return c.Mode == "active_standby" && (c.Fence == "writers" || c.Fence == "writer")
}

// StickyWriters reports whether per-user writer leases are enabled.
func (c HAConfig) StickyWriters() bool {
	return c.Mode == "sticky"
}

type ServerConfig struct {
	Hostname string `yaml:"hostname"`
}

type StorageConfig struct {
	Driver   string         `yaml:"driver"` // sqlite | postgres
	SQLite   SQLiteConfig   `yaml:"sqlite"`
	Postgres PostgresConfig `yaml:"postgres"`
}

type SQLiteConfig struct {
	Path string `yaml:"path"`
}

type PostgresConfig struct {
	DSN string `yaml:"dsn"`
}

type MailstoreConfig struct {
	Root        string            `yaml:"root"`
	ObjectStore ObjectStoreConfig `yaml:"object_store"`
}

// ObjectStoreConfig enables write-through S3-compatible maildir mirroring.
type ObjectStoreConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Endpoint     string        `yaml:"endpoint"`
	Region       string        `yaml:"region"`
	Bucket       string        `yaml:"bucket"`
	AccessKey    string        `yaml:"access_key"`
	SecretKey    string        `yaml:"secret_key"`
	Prefix       string        `yaml:"prefix"`
	PathStyle    bool          `yaml:"path_style"`
	SyncInterval time.Duration `yaml:"sync_interval"` // 0 = off; e.g. 1h runs SyncObjects in-process
}

type SMTPConfig struct {
	Submission     string              `yaml:"submission"`
	MX             string              `yaml:"mx"`
	SMTPS          string              `yaml:"smtps"`
	MaxSize        int64               `yaml:"max_size"` // bytes; 0 = default 25 MiB
	ReadTimeout    time.Duration       `yaml:"read_timeout"`
	WriteTimeout   time.Duration       `yaml:"write_timeout"`
	OutboundDirect bool                `yaml:"outbound_direct"` // MX lookup when relay.host empty
	Relay          SMTPRelayConfig     `yaml:"relay"`
	DKIM           SMTPDKIMConfig      `yaml:"dkim"`
	RateLimit      SMTPRateLimitConfig `yaml:"rate_limit"`
	Queue          SMTPQueueConfig     `yaml:"queue"`
}

// SMTPQueueConfig enables durable outbound retries and DSN bounces.
type SMTPQueueConfig struct {
	Enabled      bool          `yaml:"enabled"` // default true when outbound is configured
	Workers      int           `yaml:"workers"`
	PollInterval time.Duration `yaml:"poll_interval"`
	MaxAttempts  int           `yaml:"max_attempts"`
	BatchSize    int           `yaml:"batch_size"`
}

// SMTPRateLimitConfig limits accepted messages per remote IP and authenticated user.
type SMTPRateLimitConfig struct {
	PerIP   int           `yaml:"per_ip"`   // 0 = off
	PerUser int           `yaml:"per_user"` // 0 = off
	Window  time.Duration `yaml:"window"`   // default 1m
}

// SMTPRelayConfig is the outbound smart-host for authenticated external recipients.
type SMTPRelayConfig struct {
	Host            string `yaml:"host"` // host:port; empty disables outbound
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	DisableSTARTTLS bool   `yaml:"disable_starttls"`
}

// SMTPDKIMConfig signs outbound messages before relay.
type SMTPDKIMConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Domain         string `yaml:"domain"`
	Selector       string `yaml:"selector"`
	PrivateKeyFile string `yaml:"private_key_file"`
}

type IMAPConfig struct {
	Listen string `yaml:"listen"`
	IMAPS  string `yaml:"imaps"`
}

type POP3Config struct {
	Listen string `yaml:"listen"`
	POP3S  string `yaml:"pop3s"`
}

type ManageSieveConfig struct {
	Listen string `yaml:"listen"`
}

// LDAPConfig holds per-domain LDAP directories for hybrid authentication.
type LDAPConfig struct {
	CacheTTL time.Duration               `yaml:"cache_ttl"`
	Domains  map[string]LDAPDomainConfig `yaml:"domains"`
}

type LDAPDomainConfig struct {
	Enabled      bool             `yaml:"enabled"`
	URL          string           `yaml:"url"` // ldap:// or ldaps://
	StartTLS     bool             `yaml:"start_tls"`
	BindDN       string           `yaml:"bind_dn"`
	BindPassword string           `yaml:"bind_password"`
	BaseDN       string           `yaml:"base_dn"`
	Filter       string           `yaml:"filter"` // {email}, {user}
	AttrEmail    string           `yaml:"attr_email"`
	AttrName     string           `yaml:"attr_name"`
	Timeout      time.Duration    `yaml:"timeout"`
	Groups       LDAPGroupsConfig `yaml:"groups"`
	Sync         LDAPSyncConfig   `yaml:"sync"`
}

// LDAPGroupsConfig maps LDAP group membership to Tayga roles at login.
type LDAPGroupsConfig struct {
	Mode         string   `yaml:"mode"`          // off | memberof | search
	AttrMemberOf string   `yaml:"attr_memberof"` // default memberOf
	BaseDN       string   `yaml:"base_dn"`       // group search base (search mode)
	Filter       string   `yaml:"filter"`        // {dn} placeholder (search mode)
	AttrName     string   `yaml:"attr_name"`     // group CN attr (default cn)
	AdminGroups  []string `yaml:"admin_groups"`  // DN or CN → role admin
}

// LDAPSyncConfig periodically provisions/updates users from LDAP groups.
type LDAPSyncConfig struct {
	Enabled    bool          `yaml:"enabled"`
	Interval   time.Duration `yaml:"interval"`    // 0 = only CLI / on login
	GroupDNs   []string      `yaml:"group_dns"`   // groups whose members get mailboxes
	MemberAttr string        `yaml:"member_attr"` // default member
}

// OIDCConfig holds per-domain OpenID Connect IdP settings.
type OIDCConfig struct {
	Domains map[string]OIDCDomainConfig `yaml:"domains"`
}

type OIDCDomainConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	RedirectURL  string   `yaml:"redirect_url"`
	Scopes       []string `yaml:"scopes"`
}

type MFAConfig struct {
	Issuer          string        `yaml:"issuer"`            // TOTP issuer label
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`  // default 1h
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"` // default 720h
	ChallengeTTL    time.Duration `yaml:"challenge_ttl"`     // default 5m
	// When true, users with TOTP/WebAuthn cannot use password auth on IMAP/SMTP.
	RequireTokenForMFAUsers bool           `yaml:"require_token_for_mfa_users"`
	WebAuthn                WebAuthnConfig `yaml:"webauthn"`
}

// WebAuthnConfig configures passkey / security-key MFA (RP ID defaults from http.public_url).
type WebAuthnConfig struct {
	Enabled       bool     `yaml:"enabled"`
	RPDisplayName string   `yaml:"rp_display_name"`
	RPID          string   `yaml:"rp_id"`
	RPOrigins     []string `yaml:"rp_origins"`
}

// FlowSyncConfig controls Tayga's proprietary FlowSync engine
// (ActiveSync- and EWS-compatible wire protocols).
type FlowSyncConfig struct {
	Enabled bool `yaml:"enabled"`
}

type TLSConfig struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	// AutoGenerate writes a self-signed cert when cert/key files are missing (dev only).
	AutoGenerate bool `yaml:"auto_generate"`
}

type HTTPConfig struct {
	Listen    string `yaml:"listen"`     // plain HTTP (optional when TLSListen is set)
	TLSListen string `yaml:"tls_listen"` // HTTPS (implicit TLS), e.g. ":8443"
	// RedirectHTTPToTLS, when true and both Listen and TLSListen are set, redirects HTTP→HTTPS.
	RedirectHTTPToTLS bool     `yaml:"redirect_http_to_tls"`
	PublicURL         string   `yaml:"public_url"` // e.g. https://127.0.0.1:8443 for OIDC redirects
	Admins            []string `yaml:"admins"`     // emails allowed to call /api/v1/admin/*
}

type SeedConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Tenant   string `yaml:"tenant"`
	Domain   string `yaml:"domain"`
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

type LogConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, text
}

// Load reads and validates configuration from path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Default returns sensible defaults used before YAML overlay.
func Default() *Config {
	return &Config{
		Server: ServerConfig{Hostname: "localhost"},
		Storage: StorageConfig{
			Driver: "sqlite",
			SQLite: SQLiteConfig{Path: "./data/tayga.db"},
		},
		Mailstore: MailstoreConfig{Root: "./data/maildir"},
		SMTP: SMTPConfig{
			Submission:   ":587",
			MX:           ":25",
			SMTPS:        ":465",
			MaxSize:      25 << 20,
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 60 * time.Second,
			Queue: SMTPQueueConfig{
				Enabled:      true,
				Workers:      1,
				PollInterval: 5 * time.Second,
				MaxAttempts:  8,
				BatchSize:    10,
			},
		},
		IMAP:        IMAPConfig{Listen: ":143", IMAPS: ":993"},
		POP3:        POP3Config{Listen: ":110", POP3S: ":995"},
		ManageSieve: ManageSieveConfig{Listen: ":4190"},
		TLS: TLSConfig{
			CertFile:     "./data/certs/server.crt",
			KeyFile:      "./data/certs/server.key",
			AutoGenerate: true,
		},
		LDAP: LDAPConfig{
			CacheTTL: 5 * time.Minute,
			Domains:  map[string]LDAPDomainConfig{},
		},
		OIDC: OIDCConfig{Domains: map[string]OIDCDomainConfig{}},
		MFA: MFAConfig{
			Issuer:                  "Tayga Mail",
			AccessTokenTTL:          time.Hour,
			RefreshTokenTTL:         30 * 24 * time.Hour,
			ChallengeTTL:            5 * time.Minute,
			RequireTokenForMFAUsers: true,
			WebAuthn: WebAuthnConfig{
				Enabled: true,
			},
		},
		FlowSync: FlowSyncConfig{Enabled: true},
		HTTP: HTTPConfig{
			Listen:            ":80",
			TLSListen:         ":443",
			RedirectHTTPToTLS: true,
			PublicURL:         "https://127.0.0.1",
			Admins:            []string{"admin@example.com"},
		},
		HA: HAConfig{
			Mode: "none", LeaseTTL: 5 * time.Second, Fence: "mx",
			StickyTTL: 30 * time.Second,
		},
		Scan: ScanConfig{
			Enabled:          false,
			Backend:          "clamav",
			Action:           "quarantine",
			QuarantineFolder: "Quarantine",
			Timeout:          30 * time.Second,
			FailOpen:         false,
			ClamAV:           ClamAVConfig{Address: "127.0.0.1:3310"},
		},
		Spam: SpamConfig{
			Enabled:      false,
			Backend:      "rspamd",
			URL:          "http://127.0.0.1:11333",
			Timeout:      10 * time.Second,
			FailOpen:     true,
			Folder:       "Junk",
			FollowRspamd: true,
		},
		DKIMVerify: DKIMVerifyConfig{
			Enabled:  false,
			Action:   "tag",
			FailOpen: true,
		},
		SPF: SPFConfig{
			Enabled:  false,
			Action:   "tag",
			FailOpen: true,
		},
		DMARC: DMARCConfig{
			Enabled:  false,
			Action:   "tag",
			FailOpen: true,
			Report: DMARCReportConfig{
				Enabled:  false,
				OrgName:  "Tayga Mail",
				Interval: 24 * time.Hour,
			},
		},
		Log: LogConfig{Level: "info", Format: "json"},
	}
}

// Validate checks required fields.
func (c *Config) Validate() error {
	if c.Server.Hostname == "" {
		return fmt.Errorf("server.hostname is required")
	}
	switch c.Storage.Driver {
	case "sqlite":
		if c.Storage.SQLite.Path == "" {
			return fmt.Errorf("storage.sqlite.path is required")
		}
	case "postgres":
		if c.Storage.Postgres.DSN == "" {
			return fmt.Errorf("storage.postgres.dsn is required")
		}
	default:
		return fmt.Errorf("storage.driver must be sqlite or postgres, got %q", c.Storage.Driver)
	}
	if c.Mailstore.Root == "" {
		return fmt.Errorf("mailstore.root is required")
	}
	if c.Mailstore.ObjectStore.Enabled {
		os := c.Mailstore.ObjectStore
		if os.Endpoint == "" || os.Bucket == "" || os.AccessKey == "" || os.SecretKey == "" {
			return fmt.Errorf("mailstore.object_store: endpoint, bucket, access_key, and secret_key are required when enabled")
		}
		if os.Region == "" {
			c.Mailstore.ObjectStore.Region = "us-east-1"
		}
	}
	if c.SMTP.MaxSize <= 0 {
		c.SMTP.MaxSize = 25 << 20
	}
	if c.SMTP.ReadTimeout <= 0 {
		c.SMTP.ReadTimeout = 60 * time.Second
	}
	if c.SMTP.WriteTimeout <= 0 {
		c.SMTP.WriteTimeout = 60 * time.Second
	}
	if c.SMTP.DKIM.Enabled {
		if c.SMTP.DKIM.Domain == "" || c.SMTP.DKIM.Selector == "" || c.SMTP.DKIM.PrivateKeyFile == "" {
			return fmt.Errorf("smtp.dkim: domain, selector, and private_key_file required when enabled")
		}
	}
	if c.SMTP.RateLimit.PerIP > 0 || c.SMTP.RateLimit.PerUser > 0 {
		if c.SMTP.RateLimit.Window <= 0 {
			c.SMTP.RateLimit.Window = time.Minute
		}
	}
	if c.SMTP.Queue.Workers <= 0 {
		c.SMTP.Queue.Workers = 1
	}
	if c.SMTP.Queue.PollInterval <= 0 {
		c.SMTP.Queue.PollInterval = 5 * time.Second
	}
	if c.SMTP.Queue.MaxAttempts <= 0 {
		c.SMTP.Queue.MaxAttempts = 8
	}
	if c.SMTP.Queue.BatchSize <= 0 {
		c.SMTP.Queue.BatchSize = 10
	}
	if c.LDAP.CacheTTL <= 0 {
		c.LDAP.CacheTTL = 5 * time.Minute
	}
	if c.LDAP.Domains == nil {
		c.LDAP.Domains = map[string]LDAPDomainConfig{}
	}
	for name, d := range c.LDAP.Domains {
		if !d.Enabled {
			continue
		}
		if d.URL == "" || d.BaseDN == "" || d.Filter == "" {
			return fmt.Errorf("ldap.domains.%s: url, base_dn, and filter are required when enabled", name)
		}
		if d.AttrEmail == "" {
			d.AttrEmail = "mail"
		}
		if d.AttrName == "" {
			d.AttrName = "cn"
		}
		if d.Timeout <= 0 {
			d.Timeout = 10 * time.Second
		}
		switch strings.ToLower(strings.TrimSpace(d.Groups.Mode)) {
		case "", "off", "none", "memberof", "search":
			if d.Groups.Mode == "" {
				d.Groups.Mode = "off"
			} else {
				d.Groups.Mode = strings.ToLower(strings.TrimSpace(d.Groups.Mode))
			}
		default:
			return fmt.Errorf("ldap.domains.%s.groups.mode must be off, memberof, or search", name)
		}
		if d.Groups.AttrMemberOf == "" {
			d.Groups.AttrMemberOf = "memberOf"
		}
		if d.Groups.AttrName == "" {
			d.Groups.AttrName = "cn"
		}
		if d.Groups.Mode == "search" && d.Groups.Filter == "" {
			d.Groups.Filter = "(&(objectClass=groupOfNames)(member={dn}))"
		}
		if d.Sync.MemberAttr == "" {
			d.Sync.MemberAttr = "member"
		}
		if d.Sync.Enabled && d.Sync.Interval < 0 {
			return fmt.Errorf("ldap.domains.%s.sync.interval must be >= 0", name)
		}
		c.LDAP.Domains[name] = d
	}
	if c.OIDC.Domains == nil {
		c.OIDC.Domains = map[string]OIDCDomainConfig{}
	}
	for name, d := range c.OIDC.Domains {
		if !d.Enabled {
			continue
		}
		if d.Issuer == "" || d.ClientID == "" || d.RedirectURL == "" {
			return fmt.Errorf("oidc.domains.%s: issuer, client_id, redirect_url required when enabled", name)
		}
		if len(d.Scopes) == 0 {
			d.Scopes = []string{"openid", "email", "profile"}
		}
		c.OIDC.Domains[name] = d
	}
	if c.MFA.Issuer == "" {
		c.MFA.Issuer = "Tayga Mail"
	}
	if c.MFA.AccessTokenTTL <= 0 {
		c.MFA.AccessTokenTTL = time.Hour
	}
	if c.MFA.RefreshTokenTTL <= 0 {
		c.MFA.RefreshTokenTTL = 30 * 24 * time.Hour
	}
	if c.MFA.ChallengeTTL <= 0 {
		c.MFA.ChallengeTTL = 5 * time.Minute
	}
	if c.HTTP.PublicURL == "" {
		c.HTTP.PublicURL = "http://127.0.0.1" + c.HTTP.Listen
	}
	if c.MFA.WebAuthn.RPDisplayName == "" {
		c.MFA.WebAuthn.RPDisplayName = c.MFA.Issuer
	}
	if u, err := url.Parse(c.HTTP.PublicURL); err == nil && u.Hostname() != "" {
		if c.MFA.WebAuthn.RPID == "" {
			c.MFA.WebAuthn.RPID = u.Hostname()
		}
		if len(c.MFA.WebAuthn.RPOrigins) == 0 {
			c.MFA.WebAuthn.RPOrigins = []string{u.Scheme + "://" + u.Host}
		}
	}
	if c.MFA.WebAuthn.Enabled && (c.MFA.WebAuthn.RPID == "" || len(c.MFA.WebAuthn.RPOrigins) == 0) {
		return fmt.Errorf("mfa.webauthn: rp_id and rp_origins required when enabled (set http.public_url)")
	}
	switch c.HA.Mode {
	case "", "none":
		c.HA.Mode = "none"
	case "active_standby":
		if c.Storage.Driver != "postgres" {
			return fmt.Errorf("ha.mode active_standby requires storage.driver postgres")
		}
		if c.HA.LeaseTTL <= 0 {
			c.HA.LeaseTTL = 5 * time.Second
		}
		switch c.HA.Fence {
		case "", "mx":
			c.HA.Fence = "mx"
		case "writers", "writer":
			c.HA.Fence = "writers"
		default:
			return fmt.Errorf("ha.fence must be mx or writers, got %q", c.HA.Fence)
		}
	case "sticky":
		if c.HA.StickyTTL <= 0 {
			c.HA.StickyTTL = 30 * time.Second
		}
	default:
		return fmt.Errorf("ha.mode must be none, active_standby, or sticky, got %q", c.HA.Mode)
	}
	if c.Scan.Enabled {
		switch c.Scan.Backend {
		case "", "clamav", "exec", "none", "noop":
			if c.Scan.Backend == "" {
				c.Scan.Backend = "clamav"
			}
		default:
			return fmt.Errorf("scan.backend must be clamav, exec, or none, got %q", c.Scan.Backend)
		}
		switch c.Scan.Action {
		case "", "reject", "quarantine", "tag":
			if c.Scan.Action == "" {
				c.Scan.Action = "quarantine"
			}
		default:
			return fmt.Errorf("scan.action must be reject, quarantine, or tag, got %q", c.Scan.Action)
		}
		if c.Scan.QuarantineFolder == "" {
			c.Scan.QuarantineFolder = "Quarantine"
		}
		if c.Scan.Timeout <= 0 {
			c.Scan.Timeout = 30 * time.Second
		}
		if c.Scan.Backend == "clamav" && c.Scan.ClamAV.Address == "" {
			c.Scan.ClamAV.Address = "127.0.0.1:3310"
		}
		if c.Scan.Backend == "exec" && len(c.Scan.Exec.Command) == 0 {
			return fmt.Errorf("scan.exec.command is required when backend is exec")
		}
	}
	if c.Spam.Enabled {
		switch c.Spam.Backend {
		case "", "rspamd", "none", "noop":
			if c.Spam.Backend == "" {
				c.Spam.Backend = "rspamd"
			}
		default:
			return fmt.Errorf("spam.backend must be rspamd or none, got %q", c.Spam.Backend)
		}
		if c.Spam.URL == "" {
			c.Spam.URL = "http://127.0.0.1:11333"
		}
		if c.Spam.Timeout <= 0 {
			c.Spam.Timeout = 10 * time.Second
		}
		if c.Spam.Folder == "" {
			c.Spam.Folder = "Junk"
		}
	}
	if c.DKIMVerify.Enabled {
		switch c.DKIMVerify.Action {
		case "", "tag", "reject":
			if c.DKIMVerify.Action == "" {
				c.DKIMVerify.Action = "tag"
			}
		default:
			return fmt.Errorf("dkim_verify.action must be tag or reject, got %q", c.DKIMVerify.Action)
		}
		if c.DKIMVerify.AuthservID == "" {
			c.DKIMVerify.AuthservID = c.Server.Hostname
		}
	}
	if c.SPF.Enabled {
		switch c.SPF.Action {
		case "", "tag", "reject":
			if c.SPF.Action == "" {
				c.SPF.Action = "tag"
			}
		default:
			return fmt.Errorf("spf.action must be tag or reject, got %q", c.SPF.Action)
		}
		if c.SPF.AuthservID == "" {
			c.SPF.AuthservID = c.Server.Hostname
		}
	}
	if c.DMARC.Enabled {
		switch c.DMARC.Action {
		case "", "tag", "reject", "follow":
			if c.DMARC.Action == "" {
				c.DMARC.Action = "tag"
			}
		default:
			return fmt.Errorf("dmarc.action must be tag, reject, or follow, got %q", c.DMARC.Action)
		}
		if c.DMARC.AuthservID == "" {
			c.DMARC.AuthservID = c.Server.Hostname
		}
	}
	if c.DMARC.Report.Enabled {
		if c.DMARC.Report.OrgName == "" {
			c.DMARC.Report.OrgName = "Tayga Mail"
		}
		if c.DMARC.Report.Contact == "" {
			c.DMARC.Report.Contact = "dmarc-noreply@" + c.Server.Hostname
		}
		if c.DMARC.Report.Interval <= 0 {
			c.DMARC.Report.Interval = 24 * time.Hour
		}
	}
	return nil
}

// TLSEnabled reports whether TLS certificate files are configured.
func (c *Config) TLSEnabled() bool {
	return c.TLS.CertFile != "" && c.TLS.KeyFile != ""
}
