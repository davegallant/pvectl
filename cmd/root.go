package cmd

import (
	"github.com/spf13/cobra"
)

var debug bool
var verbose bool

// version is set at build time via -ldflags "-X github.com/davegallant/pvectl/cmd.version=..."
// (see .goreleaser.yaml) so a release binary reports its tag version without
// a per-release source edit. The default here is what an unreleased/local
// build (including `go install`) reports.
var version = "dev"

var rootCmd = &cobra.Command{
	Use:           "pvectl",
	Short:         "CLI for Proxmox VE",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "log Proxmox API request/response activity to stderr")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "show Proxmox task IDs (UPIDs) alongside action output")
}

// Execute runs the root command and returns any error encountered.
func Execute() error {
	return rootCmd.Execute()
}

// RootCmd exposes the fully-wired command tree (every subcommand's init()
// has already run by the time any importer can call this) — used by
// tools/gendocs to walk it with cobra/doc, without making rootCmd itself
// package-public.
func RootCmd() *cobra.Command {
	return rootCmd
}
