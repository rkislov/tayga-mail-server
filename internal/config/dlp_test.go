package config

import "testing"

func TestDLPConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Config)
		valid  bool
	}{
		{"disabled", func(c *Config) { c.SMTP.DLP.Enabled = false }, true},
		{"valid", func(c *Config) {}, true},
		{"no host", func(c *Config) { c.SMTP.DLP.Host = "" }, false},
		{"invalid port", func(c *Config) { c.SMTP.DLP.Host = "dlp.example:70000" }, false},
		{"no queue", func(c *Config) { c.SMTP.Queue.Enabled = false }, false},
		{"relay conflict", func(c *Config) { c.SMTP.Relay.Host = "relay.example:25" }, false},
		{"loop", func(c *Config) { c.SMTP.DLP.Host = "localhost:25" }, false},
		{"own host", func(c *Config) { c.SMTP.DLP.Host = "mail.example.com:587" }, false},
		{"password only", func(c *Config) { c.SMTP.DLP.Password = "fixture-secret" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Default()
			c.Server.Hostname = "mail.example.com"
			c.SMTP.DLP = SMTPDLPConfig{Enabled: true, Host: "dlp.example:2525"}
			tc.change(c)
			err := c.Validate()
			if (err == nil) != tc.valid {
				t.Fatal(err)
			}
		})
	}
}
