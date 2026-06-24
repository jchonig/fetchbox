package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Interval  string              `yaml:"interval"`
	Storage   map[string]*Storage `yaml:"storage"`
	Mailboxes []Mailbox           `yaml:"mailboxes"`
}

// Storage describes a named upload destination.
// URL uses webdavs:// or webdav:// with the username embedded,
// e.g. webdavs://user@host/remote.php/webdav/base/
type Storage struct {
	Type        string `yaml:"type"`
	URL         string `yaml:"url"`
	PasswordEnv string `yaml:"password_env"`
}

type Mailbox struct {
	Name        string        `yaml:"name"`
	Host        string        `yaml:"host"`
	Port        int           `yaml:"port"`
	TLS         bool          `yaml:"tls"`
	StartTLS    bool          `yaml:"starttls"`
	Username    string        `yaml:"username"`
	PasswordEnv string        `yaml:"password_env"`
	Auth        string        `yaml:"auth"` // "plain" (default) or "oauth2"
	OAuth2      *OAuth2Config `yaml:"oauth2,omitempty"`
	Folders     []Folder      `yaml:"folders"`
}

// Password returns the IMAP password, preferring the named env var and
// falling back to the Keychain (macOS) keyed by the mailbox username.
func (m *Mailbox) Password() (string, error) {
	return getSecret(m.PasswordEnv, "fetchbox", m.Username)
}

type OAuth2Config struct {
	ClientID        string `yaml:"client_id"`
	ClientSecretEnv string `yaml:"client_secret_env"`
	RefreshTokenEnv string `yaml:"refresh_token_env"`
}

type Folder struct {
	Name        string `yaml:"name"`
	Storage     string `yaml:"storage"` // key into Config.Storage
	Path        string `yaml:"path"`
	DeleteAfter bool   `yaml:"delete_after"` // delete messages after processing (vs mark seen)
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "fetchbox.yml")
}

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	if cfg.Interval == "" {
		cfg.Interval = "5m"
	}

	return &cfg, nil
}
