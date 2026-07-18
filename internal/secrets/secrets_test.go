package secrets

import (
	"errors"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func TestFakeStoreGetSet(t *testing.T) {
	store := NewFakeStore()

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

func TestStoreImplementations(t *testing.T) {
	var _ Store = NewFakeStore()
	var _ Store = KeyringStore{}
}

func withOverriddenKeyringFuncs(t *testing.T, set func(service, user, password string) error, get func(service, user string) (string, error)) {
	t.Helper()
	origSet, origGet, origTimeout := keyringSet, keyringGet, keyringTimeout
	if set != nil {
		keyringSet = set
	}
	if get != nil {
		keyringGet = get
	}
	t.Cleanup(func() {
		keyringSet, keyringGet, keyringTimeout = origSet, origGet, origTimeout
	})
}

func TestKeyringStoreSetSuccess(t *testing.T) {
	var gotService, gotUser, gotPassword string
	withOverriddenKeyringFuncs(t, func(service, user, password string) error {
		gotService, gotUser, gotPassword = service, user, password
		return nil
	}, nil)

	if err := (KeyringStore{}).Set("pve.example.com", "s3cr3t"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if gotService != service || gotUser != "pve.example.com" || gotPassword != "s3cr3t" {
		t.Errorf("underlying keyring.Set called with (%q, %q, %q)", gotService, gotUser, gotPassword)
	}
}

func TestKeyringStoreGetNotFound(t *testing.T) {
	withOverriddenKeyringFuncs(t, nil, func(service, user string) (string, error) {
		return "", keyring.ErrNotFound
	})

	if _, err := (KeyringStore{}).Get("pve.example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestKeyringStoreSetTimesOut(t *testing.T) {
	withOverriddenKeyringFuncs(t, func(service, user, password string) error {
		time.Sleep(2 * time.Second)
		return nil
	}, nil)
	keyringTimeout = 50 * time.Millisecond

	start := time.Now()
	err := (KeyringStore{}).Set("pve.example.com", "s3cr3t")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Set() error = nil, want a timeout error")
	}
	if elapsed > 1*time.Second {
		t.Errorf("Set() took %v to return, want well under the underlying call's 2s delay", elapsed)
	}
}

func TestKeyringStoreGetTimesOut(t *testing.T) {
	withOverriddenKeyringFuncs(t, nil, func(service, user string) (string, error) {
		time.Sleep(2 * time.Second)
		return "", nil
	})
	keyringTimeout = 50 * time.Millisecond

	start := time.Now()
	_, err := (KeyringStore{}).Get("pve.example.com")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Get() error = nil, want a timeout error")
	}
	if elapsed > 1*time.Second {
		t.Errorf("Get() took %v to return, want well under the underlying call's 2s delay", elapsed)
	}
}
