package secrets

import (
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

const service = "pvectl"

// keyringTimeout bounds how long KeyringStore waits for the OS keychain.
// Var (not const) so tests can shrink it. The underlying zalando/go-keyring
// calls are synchronous with no cancellation support, so on Linux in
// particular — where they go through a D-Bus Secret Service that may not
// be running (no gnome-keyring/kwallet daemon, common on minimal or
// headless setups) — a missing service can otherwise block forever with
// no feedback, the same failure mode already fixed for the HTTP client.
var keyringTimeout = 3 * time.Second

// keyringSet/keyringGet indirect the actual zalando/go-keyring calls so
// tests can simulate a hang or a specific response without touching a
// real OS keychain.
var keyringSet = keyring.Set
var keyringGet = keyring.Get

// ErrNotFound is returned when no secret is stored for the given host.
var ErrNotFound = errors.New("secret not found, run 'pvectl setup'")

// Store persists the Proxmox API token secret, keyed by cluster host.
type Store interface {
	Get(host string) (string, error)
	Set(host, secret string) error
}

// KeyringStore stores secrets in the OS keychain via zalando/go-keyring.
type KeyringStore struct{}

func (KeyringStore) Get(host string) (string, error) {
	// Snapshot the package var into a local before launching the goroutine:
	// tests override keyringGet and restore it in t.Cleanup, which fires as
	// soon as Get returns on a timeout — but the goroutine below may not
	// read keyringGet until after that restore. Reading the local instead of
	// the var avoids a data race between the cleanup's write and this read.
	get := keyringGet
	type result struct {
		secret string
		err    error
	}
	resCh := make(chan result, 1)
	go func() {
		secret, err := get(service, host)
		resCh <- result{secret, err}
	}()

	select {
	case res := <-resCh:
		if errors.Is(res.err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		if res.err != nil {
			return "", fmt.Errorf("reading secret from keyring: %w", res.err)
		}
		return res.secret, nil
	case <-time.After(keyringTimeout):
		return "", fmt.Errorf("reading secret from OS keychain timed out after %s — is a keychain/secret-service daemon running?", keyringTimeout)
	}
}

func (KeyringStore) Set(host, secret string) error {
	set := keyringSet // see Get for why we snapshot the package var first
	errCh := make(chan error, 1)
	go func() {
		errCh <- set(service, host, secret)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("writing secret to keyring: %w", err)
		}
		return nil
	case <-time.After(keyringTimeout):
		return fmt.Errorf("writing secret to OS keychain timed out after %s — is a keychain/secret-service daemon running (e.g. gnome-keyring or kwallet on Linux)?", keyringTimeout)
	}
}
