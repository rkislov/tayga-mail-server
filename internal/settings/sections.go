package settings

import (
	"fmt"

	"github.com/tayga/tms/internal/config"
)

// EditableSections are DB-backed config keys (not storage / mailstore.root).
var EditableSections = []string{
	"server",
	"smtp",
	"imap",
	"pop3",
	"managesieve",
	"http",
	"spam",
	"scan",
	"dkim_verify",
	"spf",
	"iprev",
	"helo",
	"greylist",
	"dmarc",
	"arc",
	"ldap",
	"oidc",
	"mfa",
	"flowsync",
	"ha",
	"log",
	"seed",
	"mailstore", // object_store only; root preserved from bootstrap
	"tls",       // paths / auto_generate; certs via /admin/tls|/admin/certs
	"xmpp",      // C2S / components; bots via /admin/bots
}

// RequiresRestart reports whether changing the section needs a process restart
// for listeners / HA / snapshotted SMTP policies (phase 1).
func RequiresRestart(section string) bool {
	switch section {
	case "server", "smtp", "imap", "pop3", "managesieve", "http",
		"spam", "scan", "dkim_verify", "spf", "iprev", "helo", "greylist", "dmarc", "arc",
		"ldap", "oidc", "mfa", "flowsync", "ha", "log", "seed", "mailstore", "tls", "xmpp":
		return true
	default:
		return true
	}
}

func isEditable(section string) bool {
	for _, s := range EditableSections {
		if s == section {
			return true
		}
	}
	return false
}

func getSectionPtr(cfg *config.Config, section string) (any, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	switch section {
	case "server":
		return &cfg.Server, nil
	case "smtp":
		return &cfg.SMTP, nil
	case "imap":
		return &cfg.IMAP, nil
	case "pop3":
		return &cfg.POP3, nil
	case "managesieve":
		return &cfg.ManageSieve, nil
	case "http":
		return &cfg.HTTP, nil
	case "spam":
		return &cfg.Spam, nil
	case "scan":
		return &cfg.Scan, nil
	case "dkim_verify":
		return &cfg.DKIMVerify, nil
	case "spf":
		return &cfg.SPF, nil
	case "iprev":
		return &cfg.IPRev, nil
	case "helo":
		return &cfg.Helo, nil
	case "greylist":
		return &cfg.Greylist, nil
	case "dmarc":
		return &cfg.DMARC, nil
	case "arc":
		return &cfg.ARC, nil
	case "ldap":
		return &cfg.LDAP, nil
	case "oidc":
		return &cfg.OIDC, nil
	case "mfa":
		return &cfg.MFA, nil
	case "flowsync":
		return &cfg.FlowSync, nil
	case "ha":
		return &cfg.HA, nil
	case "log":
		return &cfg.Log, nil
	case "seed":
		return &cfg.Seed, nil
	case "mailstore":
		return &cfg.Mailstore, nil
	case "tls":
		return &cfg.TLS, nil
	case "xmpp":
		return &cfg.XMPP, nil
	default:
		return nil, fmt.Errorf("unknown section %q", section)
	}
}

func cloneConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return config.Default()
	}
	// Deep-ish copy via YAML roundtrip.
	y, err := toYAMLBytes(cfg)
	if err != nil {
		cp := *cfg
		return &cp
	}
	out := config.Default()
	if err := fromYAMLBytes(y, out); err != nil {
		cp := *cfg
		return &cp
	}
	return out
}
