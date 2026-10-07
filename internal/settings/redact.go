package settings

import (
	"strings"

	"github.com/tayga/tms/internal/config"
)

const redacted = "***"

func redactConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if cfg.SMTP.Relay.Password != "" {
		cfg.SMTP.Relay.Password = redacted
	}
	if cfg.Mailstore.ObjectStore.SecretKey != "" {
		cfg.Mailstore.ObjectStore.SecretKey = redacted
	}
	if cfg.Mailstore.ObjectStore.AccessKey != "" {
		cfg.Mailstore.ObjectStore.AccessKey = redacted
	}
	if cfg.Spam.Password != "" {
		cfg.Spam.Password = redacted
	}
	for k, d := range cfg.LDAP.Domains {
		if d.BindPassword != "" {
			d.BindPassword = redacted
			cfg.LDAP.Domains[k] = d
		}
	}
	for k, d := range cfg.OIDC.Domains {
		if d.ClientSecret != "" {
			d.ClientSecret = redacted
			cfg.OIDC.Domains[k] = d
		}
	}
	for i := range cfg.XMPP.Components {
		if cfg.XMPP.Components[i].Secret != "" {
			cfg.XMPP.Components[i].Secret = redacted
		}
	}
}

// preserveSecrets copies secret fields from old into neu when neu has empty or redacted values.
func preserveSecrets(old, neu *config.Config) {
	if old == nil || neu == nil {
		return
	}
	if isSecretUnset(neu.SMTP.Relay.Password) {
		neu.SMTP.Relay.Password = old.SMTP.Relay.Password
	}
	if isSecretUnset(neu.Mailstore.ObjectStore.SecretKey) {
		neu.Mailstore.ObjectStore.SecretKey = old.Mailstore.ObjectStore.SecretKey
	}
	if isSecretUnset(neu.Mailstore.ObjectStore.AccessKey) {
		neu.Mailstore.ObjectStore.AccessKey = old.Mailstore.ObjectStore.AccessKey
	}
	if isSecretUnset(neu.Spam.Password) {
		neu.Spam.Password = old.Spam.Password
	}
	for k, d := range neu.LDAP.Domains {
		if oldD, ok := old.LDAP.Domains[k]; ok && isSecretUnset(d.BindPassword) {
			d.BindPassword = oldD.BindPassword
			neu.LDAP.Domains[k] = d
		}
	}
	for k, d := range neu.OIDC.Domains {
		if oldD, ok := old.OIDC.Domains[k]; ok && isSecretUnset(d.ClientSecret) {
			d.ClientSecret = oldD.ClientSecret
			neu.OIDC.Domains[k] = d
		}
	}
	for i := range neu.XMPP.Components {
		if !isSecretUnset(neu.XMPP.Components[i].Secret) {
			continue
		}
		if i < len(old.XMPP.Components) {
			neu.XMPP.Components[i].Secret = old.XMPP.Components[i].Secret
		}
	}
}

func isSecretUnset(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || s == redacted
}

func preserveBootstrap(bootstrap, cfg *config.Config) {
	if bootstrap == nil || cfg == nil {
		return
	}
	cfg.Storage = bootstrap.Storage
	cfg.Mailstore.Root = bootstrap.Mailstore.Root
}
