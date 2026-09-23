// Command gendocs regenerates docs/cli/ from pvectl's actual cobra command
// tree (Use/Short/Long/flags), via cobra's own doc.GenMarkdownTree — a
// mechanical flag/usage reference, not a replacement for README.md's
// narrative sections (permission tables, gotchas, the "why" behind
// defaults). Run via `just docs`; not part of the shipped pvectl binary,
// so it lives outside cmd/pvectl.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/davegallant/pvectl/cmd"
	"github.com/spf13/cobra/doc"
)

func main() {
	const dir = "docs/cli"
	// A changing generation date makes a clean docs tree appear stale in CI.
	cmd.RootCmd().DisableAutoGenTag = true
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("creating %s: %v", dir, err)
	}
	if err := doc.GenMarkdownTree(cmd.RootCmd(), dir); err != nil {
		log.Fatalf("generating docs: %v", err)
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		log.Fatalf("finding generated docs: %v", err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("reading %s: %v", path, err)
		}
		content := strings.TrimRight(string(data), "\n") + "\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			log.Fatalf("finishing %s: %v", path, err)
		}
	}
}
