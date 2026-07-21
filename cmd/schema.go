package cmd

import (
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// schemaFlag is one flag entry in `pvectl schema`'s output.
type schemaFlag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default,omitempty"`
	Usage     string `json:"usage,omitempty"`
}

// schemaCommand is one command's entry in `pvectl schema`'s output —
// its own name/use/flags plus its subcommands, recursively. Global flags
// inherited from the root (--json/--debug/--verbose) are listed once at
// the top level instead of being repeated on every node.
type schemaCommand struct {
	Name     string          `json:"name"`
	Use      string          `json:"use"`
	Aliases  []string        `json:"aliases,omitempty"`
	Short    string          `json:"short,omitempty"`
	Flags    []schemaFlag    `json:"flags,omitempty"`
	Commands []schemaCommand `json:"commands,omitempty"`
}

// pvectlSchema is the root of `pvectl schema`'s output: an agent-oriented
// description of pvectl's full command tree, for introspection without
// having to parse --help text.
type pvectlSchema struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Short       string          `json:"short,omitempty"`
	GlobalFlags []schemaFlag    `json:"globalFlags,omitempty"`
	Commands    []schemaCommand `json:"commands"`
}

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print pvectl's command tree (names, flags, descriptions) as JSON, for agent introspection",
	RunE: func(cmd *cobra.Command, args []string) error {
		return printJSON(buildSchema(rootCmd))
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}

// buildSchema walks root's command tree into a pvectlSchema. root's own
// persistent flags become GlobalFlags; every other command's flags come
// from its own LocalFlags (never inherited ones), so a global flag isn't
// repeated on each node.
func buildSchema(root *cobra.Command) pvectlSchema {
	return pvectlSchema{
		Name:        root.Name(),
		Version:     root.Version,
		Short:       root.Short,
		GlobalFlags: schemaFlags(root.PersistentFlags()),
		Commands:    schemaSubcommands(root),
	}
}

// schemaSubcommands builds sorted schemaCommand entries for each of
// parent's visible, runnable subcommands (cobra's own auto-generated
// `help` is skipped — it's not part of pvectl's own functionality).
func schemaSubcommands(parent *cobra.Command) []schemaCommand {
	var out []schemaCommand
	for _, c := range parent.Commands() {
		if c.Hidden || c.Name() == "help" {
			continue
		}
		out = append(out, schemaCommand{
			Name:     c.Name(),
			Use:      c.Use,
			Aliases:  c.Aliases,
			Short:    c.Short,
			Flags:    schemaFlags(c.LocalFlags()),
			Commands: schemaSubcommands(c),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// schemaFlags converts a pflag.FlagSet into sorted schemaFlag entries, or
// nil if it has none — VisitAll walks in lexicographical order already,
// but FlagSet doesn't guarantee that across cobra versions, so entries are
// sorted explicitly for stable output.
func schemaFlags(flags *pflag.FlagSet) []schemaFlag {
	var out []schemaFlag
	flags.VisitAll(func(f *pflag.Flag) {
		out = append(out, schemaFlag{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Type:      f.Value.Type(),
			Default:   f.DefValue,
			Usage:     f.Usage,
		})
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
