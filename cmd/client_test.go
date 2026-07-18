package cmd

import (
	"errors"
	"testing"

	"github.com/davegallant/pvectl/internal/config"
	"github.com/davegallant/pvectl/internal/secrets"
)

func TestLoadClientNoConfig(t *testing.T) {
	orig := config.ConfigDir
	tmpDir := t.TempDir()
	config.ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { config.ConfigDir = orig })

	_, err := loadClient()
	if !errors.Is(err, config.ErrNotFound) {
		t.Errorf("loadClient() error = %v, want config.ErrNotFound", err)
	}
}

func TestLoadClientNoSecret(t *testing.T) {
	orig := config.ConfigDir
	tmpDir := t.TempDir()
	config.ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { config.ConfigDir = orig })
	keyringStore = secrets.NewFakeStore()
	t.Cleanup(func() { keyringStore = secrets.KeyringStore{} })

	if err := config.Save(&config.Config{Host: "https://pve.example.com:8006", TokenID: "user@pve!pvectl"}); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}

	_, err := loadClient()
	if !errors.Is(err, secrets.ErrNotFound) {
		t.Errorf("loadClient() error = %v, want secrets.ErrNotFound", err)
	}
}

func TestLoadClientSuccess(t *testing.T) {
	orig := config.ConfigDir
	tmpDir := t.TempDir()
	config.ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { config.ConfigDir = orig })
	fake := secrets.NewFakeStore()
	keyringStore = fake
	t.Cleanup(func() { keyringStore = secrets.KeyringStore{} })

	host := "https://pve.example.com:8006"
	if err := config.Save(&config.Config{Host: host, TokenID: "user@pve!pvectl"}); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}
	if err := fake.Set(host, "s3cr3t"); err != nil {
		t.Fatalf("fake.Set() error = %v", err)
	}

	client, err := loadClient()
	if err != nil {
		t.Fatalf("loadClient() error = %v", err)
	}
	if client == nil {
		t.Error("loadClient() returned nil client")
	}
}

func TestLoadClientUsesFileBackendWhenConfigured(t *testing.T) {
	orig := config.ConfigDir
	tmpDir := t.TempDir()
	config.ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { config.ConfigDir = orig })

	fakeFile := secrets.NewFakeStore()
	fileStore = fakeFile
	t.Cleanup(func() { fileStore = secrets.FileStore{} })

	fakeKeyring := secrets.NewFakeStore()
	keyringStore = fakeKeyring
	t.Cleanup(func() { keyringStore = secrets.KeyringStore{} })

	host := "https://pve.example.com:8006"
	if err := config.Save(&config.Config{Host: host, TokenID: "user@pve!pvectl", SecretBackend: "file"}); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}
	// Secret lives only in the file backend; the keyring fake is left
	// empty, so success here proves loadClient() read from the backend
	// recorded in config, not the keyring.
	if err := fakeFile.Set(host, "s3cr3t"); err != nil {
		t.Fatalf("fakeFile.Set() error = %v", err)
	}

	client, err := loadClient()
	if err != nil {
		t.Fatalf("loadClient() error = %v", err)
	}
	if client == nil {
		t.Error("loadClient() returned nil client")
	}
}

func TestFriendlyLoginError(t *testing.T) {
	got := friendlySetupError(config.ErrNotFound)
	if got.Error() != "not set up, run 'pvectl setup'" {
		t.Errorf("friendlySetupError(config.ErrNotFound) = %q", got.Error())
	}

	got = friendlySetupError(secrets.ErrNotFound)
	if got.Error() != "not set up, run 'pvectl setup'" {
		t.Errorf("friendlySetupError(secrets.ErrNotFound) = %q", got.Error())
	}

	other := errors.New("boom")
	if got := friendlySetupError(other); got != other {
		t.Errorf("friendlySetupError(other) = %v, want unchanged", got)
	}
}
