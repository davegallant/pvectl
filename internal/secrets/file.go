package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileStoreDir returns the directory the file-based secret store's file
// lives under. Overridable in tests to avoid touching the real OS config
// directory.
var FileStoreDir = defaultFileStoreDir

func defaultFileStoreDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving config dir: %w", err)
	}
	return filepath.Join(dir, "pvectl"), nil
}

func filePath() (string, error) {
	dir, err := FileStoreDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "secrets.json"), nil
}

// FileStore stores secrets in a local file (permissions 0600), keyed by
// host. This is a less secure fallback for systems with no working OS
// keychain (e.g. no D-Bus Secret Service running) — the secret is stored
// in cleartext, protected only by file permissions.
type FileStore struct{}

func readSecretsFile() (map[string]string, error) {
	path, err := filePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("reading secrets file: %w", err)
	}

	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing secrets file: %w", err)
	}
	if m == nil {
		m = map[string]string{}
	}
	return m, nil
}

func writeSecretsFile(m map[string]string) error {
	path, err := filePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating secrets dir: %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding secrets file: %w", err)
	}

	return os.WriteFile(path, data, 0o600)
}

func (FileStore) Get(host string) (string, error) {
	m, err := readSecretsFile()
	if err != nil {
		return "", err
	}
	secret, ok := m[host]
	if !ok {
		return "", ErrNotFound
	}
	return secret, nil
}

func (FileStore) Set(host, secret string) error {
	m, err := readSecretsFile()
	if err != nil {
		return err
	}
	m[host] = secret
	return writeSecretsFile(m)
}
