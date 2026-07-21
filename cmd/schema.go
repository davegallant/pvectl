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

// Mutation levels for a command's "mutation" annotation (see
// mutationAnnotation): whether running it can change cluster/local state,
// and if so, whether that change carries destructive-grade risk — either
// irreversible data loss/a point of no return (delete, rollback, restore-
// overwrite, template conversion) or an unbounded blast radius (`ct
// exec`/`ct enter`/`qm enter` hand off to an arbitrary command or an
// interactive shell, which can do anything up to and including deleting
// the guest's own filesystem — worse than any single destructive API
// call, so it gets the same tier). Group commands with no RunE of their
// own (e.g. `ct`, `ct backups`) carry no annotation and report an empty
// Mutation in schema output.
const (
	mutationSafe        = "safe"        // read-only, never changes state
	mutationMutating    = "mutating"    // changes state, but reversibly/routinely, with a bounded effect
	mutationDestructive = "destructive" // irreversible, a point of no return, or an unbounded blast radius
)

// mutationAnnotationKey is the cobra Command.Annotations key
// mutationAnnotation sets and schemaSubcommands reads back.
const mutationAnnotationKey = "mutation"

// mutationAnnotation builds the Annotations map a command's literal sets
// to record its mutation level for `pvectl schema`.
func mutationAnnotation(level string) map[string]string {
	return map[string]string{mutationAnnotationKey: level}
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
	Mutation string          `json:"mutation,omitempty"`
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
	Use:         "schema",
	Short:       "Print pvectl's command tree (names, flags, descriptions) as JSON, for agent introspection",
	Annotations: mutationAnnotation(mutationSafe),
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
		// Every pvectl-owned runnable command sets its own mutation
		// annotation explicitly (see mutationSafe/mutationMutating/
		// mutationDestructive) — the only runnable commands that can reach
		// here without one are cobra's own auto-generated ones (e.g.
		// `completion bash`/`zsh`/...), which only ever read local shell
		// config and never touch the cluster.
		mutation := c.Annotations[mutationAnnotationKey]
		if mutation == "" && c.Runnable() {
			mutation = mutationSafe
		}
		out = append(out, schemaCommand{
			Name:     c.Name(),
			Use:      c.Use,
			Aliases:  c.Aliases,
			Short:    c.Short,
			Mutation: mutation,
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
