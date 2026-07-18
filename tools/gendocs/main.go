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

	"github.com/davegallant/pvectl/cmd"
	"github.com/spf13/cobra/doc"
)

func main() {
	const dir = "docs/cli"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("creating %s: %v", dir, err)
	}
	if err := doc.GenMarkdownTree(cmd.RootCmd(), dir); err != nil {
		log.Fatalf("generating docs: %v", err)
	}
}
