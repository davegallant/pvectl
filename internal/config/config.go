package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is pvectl's non-secret, on-disk configuration.
type Config struct {
	Host               string `yaml:"host"`
	TokenID            string `yaml:"token_id"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	// SecretBackend records which secrets.Store backend the token secret
	// was saved with ("keyring" or "file"), so later commands read it
	// back from the same place without re-probing a possibly-hanging OS
	// keychain. Empty is treated as "keyring" for configs written before
	// this field existed.
	SecretBackend string `yaml:"secret_backend,omitempty"`
	// ConsoleMethod controls how `ct enter`/`qm enter` reach a guest's
	// console. "" or "ssh" (default) shells out to `ssh node pct enter`/
	// `qm terminal`. "api" opens Proxmox's termproxy websocket directly
	// using the stored API token, so no SSH access to the node is needed.
	// Empty is treated as "ssh" for configs written before this field
	// existed, same pattern as SecretBackend.
	ConsoleMethod string `yaml:"console_method,omitempty"`
}

// ErrNotFound is returned by Load when no config file exists yet.
var ErrNotFound = errors.New("config not found, run 'pvectl setup'")

// ConfigDir returns the directory pvectl's config file lives under.
// Overridable in tests to avoid touching the real OS config directory
// (os.UserConfigDir ignores XDG_CONFIG_HOME on Darwin, so t.Setenv alone
// can't isolate tests there).
var ConfigDir = defaultConfigDir

func defaultConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving config dir: %w", err)
	}
	return filepath.Join(dir, "pvectl"), nil
}

// Path returns the config file path: <ConfigDir>/config.yaml.
func Path() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	path, err := Path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	return os.WriteFile(path, data, 0o600)
}
