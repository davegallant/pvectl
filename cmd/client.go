package cmd

import (
	"errors"
	"fmt"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/davegallant/pvectl/internal/config"
	"github.com/davegallant/pvectl/internal/secrets"
)

var keyringStore secrets.Store = secrets.KeyringStore{}
var fileStore secrets.Store = secrets.FileStore{}

// secretStoreFor returns the secrets.Store that matches how cfg's secret
// was saved. Empty/"keyring" (the default, including configs written
// before this field existed) uses the OS keychain; "file" uses the
// fallback file store chosen at setup time when the keychain wasn't
// available. Using the recorded backend directly — rather than probing
// the keyring on every command — avoids re-eating its timeout on a
// system where it's known not to work.
func secretStoreFor(cfg *config.Config) secrets.Store {
	if cfg.SecretBackend == "file" {
		return fileStore
	}
	return keyringStore
}

// loadClient builds an api.Client from the stored config and secret. It
// returns config.ErrNotFound or secrets.ErrNotFound, unwrapped, if either
// is missing — callers should pass the error through friendlySetupError.
func loadClient() (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	secret, err := secretStoreFor(cfg).Get(cfg.Host)
	if err != nil {
		return nil, err
	}
	client := api.NewClient(cfg.Host, cfg.TokenID, secret, cfg.InsecureSkipVerify)
	client.SetDebug(debug)
	return client, nil
}

// friendlySetupError rewrites missing-credential errors into a single,
// actionable message. Other errors pass through unchanged.
func friendlySetupError(err error) error {
	if errors.Is(err, config.ErrNotFound) || errors.Is(err, secrets.ErrNotFound) {
		return fmt.Errorf("not set up, run 'pvectl setup'")
	}
	return err
}
