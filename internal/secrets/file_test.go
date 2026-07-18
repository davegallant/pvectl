package secrets

import (
	"errors"
	"testing"
)

func TestFileStoreGetSet(t *testing.T) {
	tmpDir := t.TempDir()
	FileStoreDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { FileStoreDir = defaultFileStoreDir })

	store := FileStore{}

	if _, err := store.Get("pve.example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() before Set() error = %v, want ErrNotFound", err)
	}

	if err := store.Set("pve.example.com", "s3cr3t"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := store.Get("pve.example.com")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != "s3cr3t" {
		t.Errorf("Get() = %q, want %q", got, "s3cr3t")
	}
}

func TestFileStorePersistsMultipleHosts(t *testing.T) {
	tmpDir := t.TempDir()
	FileStoreDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { FileStoreDir = defaultFileStoreDir })

	store := FileStore{}
	if err := store.Set("host-a.example.com", "secret-a"); err != nil {
		t.Fatalf("Set(host-a) error = %v", err)
	}
	if err := store.Set("host-b.example.com", "secret-b"); err != nil {
		t.Fatalf("Set(host-b) error = %v", err)
	}

	gotA, err := store.Get("host-a.example.com")
	if err != nil || gotA != "secret-a" {
		t.Errorf("Get(host-a) = (%q, %v), want (secret-a, nil)", gotA, err)
	}
	gotB, err := store.Get("host-b.example.com")
	if err != nil || gotB != "secret-b" {
		t.Errorf("Get(host-b) = (%q, %v), want (secret-b, nil)", gotB, err)
	}
}

func TestFileStoreImplementsStore(t *testing.T) {
	var _ Store = FileStore{}
}
