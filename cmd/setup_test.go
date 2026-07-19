package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/config"
	"github.com/davegallant/pvectl/internal/secrets"
)

func TestPromptWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue string
		example      string
		want         string
	}{
		{"blank input keeps default", "\n", "old-value", "e.g. foo", "old-value"},
		{"non-blank input overrides default", "new-value\n", "old-value", "e.g. foo", "new-value"},
		{"blank input with only an example stays blank", "\n", "", "e.g. foo", ""},
		{"blank input with neither stays blank", "\n", "", "", ""},
		{"input is trimmed", "  new-value  \n", "old-value", "e.g. foo", "new-value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			if got := promptWithDefault(reader, "label", tt.defaultValue, tt.example); got != tt.want {
				t.Errorf("promptWithDefault() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPromptYesNoDefault(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue bool
		want         bool
	}{
		{"blank keeps default true", "\n", true, true},
		{"blank keeps default false", "\n", false, false},
		{"y overrides default false", "y\n", false, true},
		{"yes overrides default false", "yes\n", false, true},
		{"n overrides default true", "n\n", true, false},
		{"uppercase Y overrides default false", "Y\n", false, true},
		{"gibberish keeps default", "maybe\n", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			if got := promptYesNoDefault(reader, "label", tt.defaultValue); got != tt.want {
				t.Errorf("promptYesNoDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunSetupSuccess(t *testing.T) {
	orig := config.ConfigDir
	tmpDir := t.TempDir()
	config.ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { config.ConfigDir = orig })
	fake := secrets.NewFakeStore()
	keyringStore = fake
	t.Cleanup(func() { keyringStore = secrets.KeyringStore{} })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"version": "8.2.1"}})
	}))
	defer server.Close()

	if err := runSetup(server.URL, "user@pve!pvectl", "s3cr3t", true, false); err != nil {
		t.Fatalf("runSetup() error = %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.Host != server.URL || cfg.TokenID != "user@pve!pvectl" {
		t.Errorf("saved config = %+v", cfg)
	}

	got, err := fake.Get(server.URL)
	if err != nil {
		t.Fatalf("fake.Get() error = %v", err)
	}
	if got != "s3cr3t" {
		t.Errorf("stored secret = %q, want s3cr3t", got)
	}
}

func TestRunSetupInvalidCredentials(t *testing.T) {
	orig := config.ConfigDir
	tmpDir := t.TempDir()
	config.ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { config.ConfigDir = orig })
	keyringStore = secrets.NewFakeStore()
	t.Cleanup(func() { keyringStore = secrets.KeyringStore{} })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "authentication failed"})
	}))
	defer server.Close()

	if err := runSetup(server.URL, "user@pve!pvectl", "wrong", true, false); err == nil {
		t.Fatal("runSetup() error = nil, want error for invalid credentials")
	}

	if _, err := config.Load(); err == nil {
		t.Error("config.Load() error = nil, want config NOT saved after failed setup")
	}
}

// failingStore is a secrets.Store test double that always returns err,
// used to simulate an unavailable OS keychain or file store.
type failingStore struct{ err error }

func (f failingStore) Get(host string) (string, error) { return "", f.err }
func (f failingStore) Set(host, secret string) error   { return f.err }

func TestRunSetupFallsBackToFileStoreWhenKeyringFails(t *testing.T) {
	orig := config.ConfigDir
	tmpDir := t.TempDir()
	config.ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { config.ConfigDir = orig })

	keyringStore = failingStore{err: errors.New("keyring unavailable")}
	t.Cleanup(func() { keyringStore = secrets.KeyringStore{} })
	fakeFile := secrets.NewFakeStore()
	fileStore = fakeFile
	t.Cleanup(func() { fileStore = secrets.FileStore{} })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"version": "8.2.1"}})
	}))
	defer server.Close()

	if err := runSetup(server.URL, "user@pve!pvectl", "s3cr3t", true, false); err != nil {
		t.Fatalf("runSetup() error = %v, want fallback to succeed", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.SecretBackend != "file" {
		t.Errorf("cfg.SecretBackend = %q, want %q", cfg.SecretBackend, "file")
	}

	got, err := fakeFile.Get(server.URL)
	if err != nil {
		t.Fatalf("fakeFile.Get() error = %v", err)
	}
	if got != "s3cr3t" {
		t.Errorf("stored secret in file backend = %q, want s3cr3t", got)
	}
}

func TestRunSetupFailsWhenKeyringAndFileBothFail(t *testing.T) {
	orig := config.ConfigDir
	tmpDir := t.TempDir()
	config.ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { config.ConfigDir = orig })

	keyringStore = failingStore{err: errors.New("keyring unavailable")}
	t.Cleanup(func() { keyringStore = secrets.KeyringStore{} })
	fileStore = failingStore{err: errors.New("disk full")}
	t.Cleanup(func() { fileStore = secrets.FileStore{} })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"version": "8.2.1"}})
	}))
	defer server.Close()

	if err := runSetup(server.URL, "user@pve!pvectl", "s3cr3t", true, false); err == nil {
		t.Fatal("runSetup() error = nil, want error when both stores fail")
	}
}
