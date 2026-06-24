package main

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	yaml := `
interval: 10m
storage:
  nextcloud:
    type: webdav
    url: webdavs://davuser@dav.example.com/webdav/
    password_env: WEBDAV_PASSWORD
mailboxes:
  - name: TestBox
    host: localhost
    port: 1143
    username: test@example.com
    password_env: TEST_PASSWORD
    folders:
      - name: INBOX
        storage: nextcloud
        path: /attachments/
`
	f, err := os.CreateTemp("", "fetchbox-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := loadConfig(f.Name())
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if cfg.Interval != "10m" {
		t.Errorf("interval: got %q, want %q", cfg.Interval, "10m")
	}
	if len(cfg.Storage) != 1 {
		t.Fatalf("storage: got %d, want 1", len(cfg.Storage))
	}
	if cfg.Storage["nextcloud"].Type != "webdav" {
		t.Errorf("storage type: got %q", cfg.Storage["nextcloud"].Type)
	}
	if len(cfg.Mailboxes) != 1 {
		t.Fatalf("mailboxes: got %d, want 1", len(cfg.Mailboxes))
	}
	mb := cfg.Mailboxes[0]
	if mb.Name != "TestBox" {
		t.Errorf("mailbox name: got %q", mb.Name)
	}
	if mb.Host != "localhost" {
		t.Errorf("host: got %q", mb.Host)
	}
	if len(mb.Folders) != 1 {
		t.Fatalf("folders: got %d, want 1", len(mb.Folders))
	}
	if mb.Folders[0].Storage != "nextcloud" {
		t.Errorf("folder storage: got %q", mb.Folders[0].Storage)
	}
	if mb.Folders[0].Path != "/attachments/" {
		t.Errorf("folder path: got %q", mb.Folders[0].Path)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	f, err := os.CreateTemp("", "fetchbox-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("mailboxes: []\n")
	f.Close()

	cfg, err := loadConfig(f.Name())
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Interval != "5m" {
		t.Errorf("default interval: got %q, want %q", cfg.Interval, "5m")
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := loadConfig("/nonexistent/path/fetchbox.yml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestMailboxPassword(t *testing.T) {
	t.Setenv("MY_PASSWORD", "s3cr3t")
	mb := Mailbox{Username: "user@example.com", PasswordEnv: "MY_PASSWORD"}
	got, err := mb.Password()
	if err != nil {
		t.Fatalf("Password(): unexpected error: %v", err)
	}
	if got != "s3cr3t" {
		t.Errorf("Password(): got %q, want %q", got, "s3cr3t")
	}
}
