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

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", `output format: "table" or "json"`)
	rootCmd.PersistentPreRunE = validateOutputFormat
}

// validateOutputFormat rejects any --output value other than "table"
// (also the default, so an omitted flag behaves the same as an explicit
// `-o table`) or "json" before a command runs, and sets jsonOutput
// accordingly — same fail-fast discipline as --method's validation in
// console.go.
func validateOutputFormat(cmd *cobra.Command, args []string) error {
	switch outputFormat {
	case "table":
		jsonOutput = false
	case "json":
		jsonOutput = true
	default:
		return fmt.Errorf(`invalid --output %q: must be "table" or "json"`, outputFormat)
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
