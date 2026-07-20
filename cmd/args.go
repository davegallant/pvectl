package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// requireArgs returns a cobra.PositionalArgs that requires exactly
// len(names) positional args, naming whichever are missing instead of
// cobra's default "accepts N arg(s), received M". It's a drop-in
// replacement for cobra.ExactArgs(len(names)).
func requireArgs(names ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < len(names) {
			missing := names[len(args):]
			return fmt.Errorf("missing required argument(s): %s", strings.Join(missing, ", "))
		}
		if len(args) > len(names) {
			return fmt.Errorf("accepts %d arg(s), received %d", len(names), len(args))
		}
		return nil
	}
}

// requireMinArgs is requireArgs' counterpart for commands that accept
// trailing variadic args beyond the named ones (e.g. `exec <target> --
// <command> [args...]`). It's a drop-in replacement for
// cobra.MinimumNArgs(len(names)).
func requireMinArgs(names ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < len(names) {
			missing := names[len(args):]
			return fmt.Errorf("missing required argument(s): %s", strings.Join(missing, ", "))
		}
		return nil
	}
}
