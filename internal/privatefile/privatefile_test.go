package privatefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFailedReplaceLeavesTargetAndCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config.yaml")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(target, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(target, []byte("new")); err == nil {
		t.Fatal("Write() succeeded replacing a nonempty directory")
	}
	got, err := os.ReadFile(sentinel)
	if err != nil || string(got) != "keep" {
		t.Errorf("sentinel = (%q, %v), want keep", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want only original target", len(entries))
	}
}
