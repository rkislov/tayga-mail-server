package config

import (
	"fmt"
	"os"
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
	TLS         TLSConfig         `yaml:"tls"`
	HTTP        HTTPConfig        `yaml:"http"`
	Seed        SeedConfig        `yaml:"seed"`
	Log         LogConfig         `yaml:"log"`
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
	Root string `yaml:"root"`
}

type SMTPConfig struct {
	Submission   string        `yaml:"submission"`
	MX           string        `yaml:"mx"`
	SMTPS        string        `yaml:"smtps"`
	MaxSize      int64         `yaml:"max_size"` // bytes; 0 = default 25 MiB
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
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
	Enabled      bool          `yaml:"enabled"`
	URL          string        `yaml:"url"` // ldap:// or ldaps://
	StartTLS     bool          `yaml:"start_tls"`
	BindDN       string        `yaml:"bind_dn"`
	BindPassword string        `yaml:"bind_password"`
	BaseDN       string        `yaml:"base_dn"`
	Filter       string        `yaml:"filter"` // {email}, {user}
	AttrEmail    string        `yaml:"attr_email"`
	AttrName     string        `yaml:"attr_name"`
	Timeout      time.Duration `yaml:"timeout"`
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
	// When true, users with TOTP enabled cannot use password auth on IMAP/SMTP.
	RequireTokenForMFAUsers bool `yaml:"require_token_for_mfa_users"`
}

type TLSConfig struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type HTTPConfig struct {
	Listen    string `yaml:"listen"`
	PublicURL string `yaml:"public_url"` // e.g. http://127.0.0.1:8080 for OIDC redirects
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
			Submission:   ":1587",
			MX:           ":1025",
			SMTPS:        "",
			MaxSize:      25 << 20,
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 60 * time.Second,
		},
		IMAP:        IMAPConfig{Listen: ":1143"},
		POP3:        POP3Config{Listen: ":1110"},
		ManageSieve: ManageSieveConfig{Listen: ":14190"},
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
		},
		HTTP: HTTPConfig{Listen: ":8080", PublicURL: "http://127.0.0.1:8080"},
		Log:  LogConfig{Level: "info", Format: "json"},
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
	if c.SMTP.MaxSize <= 0 {
		c.SMTP.MaxSize = 25 << 20
	}
	if c.SMTP.ReadTimeout <= 0 {
		c.SMTP.ReadTimeout = 60 * time.Second
	}
	if c.SMTP.WriteTimeout <= 0 {
		c.SMTP.WriteTimeout = 60 * time.Second
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
	return nil
}

// TLSEnabled reports whether TLS certificate files are configured.
func (c *Config) TLSEnabled() bool {
	return c.TLS.CertFile != "" && c.TLS.KeyFile != ""
}
