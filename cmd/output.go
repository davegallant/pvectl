package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// outputFormat backs the global `--output`/`-o` flag (kubectl/docker/gh's
// convention over a bare `--json` boolean, so a future format — e.g.
// "yaml" — has somewhere to go without a second flag). Empty means the
// default table/text output.
var outputFormat string

// jsonOutput reports whether --output json was requested: list-style
// commands print their underlying data as indented JSON instead of a
// table when true.
var jsonOutput bool

// Commands that produce a complete JSON document when --output json is set.
// API calls and schema already produce JSON regardless of the output flag.
var jsonCommands = map[string]bool{
	"pvectl ct list": true, "pvectl qm list": true,
	"pvectl ct summary": true, "pvectl qm summary": true,
	"pvectl ct config view": true, "pvectl qm config view": true,
	"pvectl ct backups list": true, "pvectl qm backups list": true,
	"pvectl ct snapshots list": true, "pvectl qm snapshots list": true,
	"pvectl nodes list": true, "pvectl storage list": true,
	"pvectl tasks list": true, "pvectl tasks status": true,
	"pvectl tasks logs": true, "pvectl templates list": true,
	"pvectl iso list": true, "pvectl schema": true,
	"pvectl api get": true, "pvectl api post": true,
	"pvectl api put": true, "pvectl api delete": true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", `output format: "table" or "json" (supported read commands and raw API)`)
	rootCmd.PersistentPreRunE = validateOutputFormat
}

// validateOutputFormat rejects any --output value other than "table"
// (also the default, so an omitted flag behaves the same as an explicit
// `-o table`) or "json" before a command runs, and sets jsonOutput
// accordingly. JSON is accepted only for commands that emit one complete
// JSON document. Watches require a terminal because they redraw in place.
func validateOutputFormat(cmd *cobra.Command, args []string) error {
	if taskWaitTimeout < 0 {
		return fmt.Errorf("--wait-timeout must be zero or positive")
	}
	switch outputFormat {
	case "table":
		jsonOutput = false
	case "json":
		jsonOutput = true
		if cmd != rootCmd && !jsonCommands[cmd.CommandPath()] {
			return fmt.Errorf("--output json is not supported by %s", cmd.CommandPath())
		}
		if cmd == tasksListCmd && tasksWatch {
			return fmt.Errorf("--output json cannot be combined with --watch")
		}
	default:
		return fmt.Errorf(`invalid --output %q: must be "table" or "json"`, outputFormat)
	}
	if (cmd == tasksListCmd && tasksWatch || cmd == statusCmd && statusWatch) && !isInteractive() {
		return fmt.Errorf("--watch requires a terminal")
	}
	return nil
}

// printJSON marshals v as indented JSON to stdout. v should be the same
// already-fetched data a command's table renderer would consume, so JSON
// and table output never drift apart on the same underlying fetch.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}
