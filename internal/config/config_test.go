package config

import (
	"errors"
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

func TestLoadNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { ConfigDir = defaultConfigDir })

	_, err := Load()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Load() error = %v, want ErrNotFound", err)
	}
}
