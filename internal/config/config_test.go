package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { ConfigDir = defaultConfigDir })

	want := &Config{
		Host:               "https://pve.example.com:8006",
		TokenID:            "user@pve!pvectl",
		InsecureSkipVerify: true,
		SecretBackend:      "file",
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if *got != *want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestSaveReplacesPermissiveFileWithPrivatePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { ConfigDir = defaultConfigDir })
	path := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(path, []byte("old configuration"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(&Config{Host: "https://pve.example.com:8006"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("config mode = %o, want 600", info.Mode().Perm())
	}
	got, err := Load()
	if err != nil || got.Host != "https://pve.example.com:8006" {
		t.Errorf("Load() = (%+v, %v), want replacement config", got, err)
	}
}

func TestLoadNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { ConfigDir = defaultConfigDir })

	_, err := Load()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Load() error = %v, want ErrNotFound", err)
	}
}
