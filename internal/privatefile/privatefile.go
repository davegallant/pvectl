// Package privatefile replaces a local file through a private temporary file.
package privatefile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write replaces path with data. The caller creates the parent directory.
// A failed write leaves the old destination in place and removes the temp file.
func Write(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pvectl-*")
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	defer func() { _ = tmp.Close() }()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("setting temporary file permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("writing temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("syncing temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}
