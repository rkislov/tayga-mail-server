package climenu

import (
	"os"
	"path/filepath"

	"github.com/tayga/tms/internal/config"
	"gopkg.in/yaml.v3"
)

// writeConfigYAML writes a minimal bootstrap YAML from a runtime config.
func writeConfigYAML(path string, cfg *config.Config) error {
	type minimal struct {
		Server struct {
			Hostname   string `yaml:"hostname"`
			SecretsKey string `yaml:"secrets_key,omitempty"`
		} `yaml:"server"`
		Storage   config.StorageConfig `yaml:"storage"`
		Mailstore struct {
			Root string `yaml:"root"`
		} `yaml:"mailstore"`
		Seed config.SeedConfig `yaml:"seed"`
		HTTP struct {
			PublicURL string   `yaml:"public_url,omitempty"`
			Admins    []string `yaml:"admins,omitempty"`
		} `yaml:"http,omitempty"`
		TLS struct {
			CertFile     string `yaml:"cert_file,omitempty"`
			KeyFile      string `yaml:"key_file,omitempty"`
			CertsDir     string `yaml:"certs_dir,omitempty"`
			AutoGenerate bool   `yaml:"auto_generate"`
		} `yaml:"tls,omitempty"`
	}
	var out minimal
	out.Server.Hostname = cfg.Server.Hostname
	out.Server.SecretsKey = cfg.Server.SecretsKey
	out.Storage = cfg.Storage
	out.Mailstore.Root = cfg.Mailstore.Root
	out.Seed = cfg.Seed
	out.HTTP.PublicURL = cfg.HTTP.PublicURL
	out.HTTP.Admins = cfg.HTTP.Admins
	out.TLS.CertFile = cfg.TLS.CertFile
	out.TLS.KeyFile = cfg.TLS.KeyFile
	out.TLS.CertsDir = cfg.TLS.CertsDir
	out.TLS.AutoGenerate = cfg.TLS.AutoGenerate
	raw, err := yaml.Marshal(&out)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil && filepath.Dir(path) != "." {
		return err
	}
	header := "# Tayga Mail — bootstrap (written by tayga menu transfer)\n"
	return os.WriteFile(path, append([]byte(header), raw...), 0o640)
}
