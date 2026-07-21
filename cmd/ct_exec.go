package cmd

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/davegallant/pvectl/internal/ssh"
	"github.com/spf13/cobra"
)

// execCompletionTimeout bounds how long a single Tab press may block on
// the remote `ls` used by completeExecArgs — a stalled or unreachable node
// must degrade to "no completions", not hang the shell.
const execCompletionTimeout = 3 * time.Second

var ctExecCmd = &cobra.Command{
	Use:               "exec <name-or-vmid> -- <command> [args...]",
	Short:             "Run a command inside a container, non-interactively (requires SSH)",
	Args:              requireMinArgs("name-or-vmid", "command"),
	ValidArgsFunction: completeExecArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		c, err := resolveContainer(client, args)
		if err != nil {
			return err
		}
		return ssh.Exec(c.Node, c.VMID, args[1:])
	},
}

func init() {
	ctCmd.AddCommand(ctExecCmd)
}

// completeExecArgs is ctExecCmd's ValidArgsFunction. args[0] (the
// container name) completes the same way every other ct command does, via
// completeContainerNames. args[1] is the remote command name itself
// (e.g. `cat`) — left uncompleted, since there's no way to know what
// executables exist in the container's PATH without yet another remote
// round trip, and that's out of scope here. Every arg after that
// (args[2:], the remote command's own arguments) gets remote path
// completion: the same "SSH out and ls the remote directory" technique
// bash-completion's _scp/_rsync functions use for remote paths, run fresh
// on every Tab press (no caching) with execCompletionTimeout bounding the
// round trip.
func completeExecArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return completeContainerNames(cmd, args, toComplete)
	}
	if len(args) == 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client, err := loadClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	c, err := resolveContainer(client, args)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	dir, prefix := path.Split(toComplete)
	listDir := dir
	if listDir == "" {
		listDir = "."
	}

	ctx, cancel := context.WithTimeout(context.Background(), execCompletionTimeout)
	defer cancel()
	entries := ssh.ListDir(ctx, c.Node, c.VMID, listDir)

	candidates := execCandidates(entries, dir, prefix)
	directive := cobra.ShellCompDirectiveNoFileComp
	if needsNoSpace(candidates) {
		directive |= cobra.ShellCompDirectiveNoSpace
	}
	return candidates, directive
}

// execCandidates filters dir's already-fetched remote entries down to
// those matching prefix, reattaching dir so each candidate is the full
// path the user would see completed — split out from completeExecArgs so
// the filtering logic is unit-testable without a live SSH connection, same
// pattern as containerNames/renderContainerList.
func execCandidates(entries []string, dir, prefix string) []string {
	var candidates []string
	for _, entry := range entries {
		if strings.HasPrefix(entry, prefix) {
			candidates = append(candidates, dir+entry)
		}
	}
	return candidates
}

// needsNoSpace reports whether completion should suppress the trailing
// space cobra/the shell would otherwise insert after a completed argument.
// This is only wanted when the user might keep typing past what was
// completed, i.e. a directory entry (trailing "/", from ls -1p) that
// invites a deeper path. A single complete regular-file match doesn't need
// it, and forcing NoSpace there is actively harmful: cobra's generated
// fish script reacts to "exactly one candidate + NoSpace" by injecting a
// decoy candidate with a "." appended (to stop fish auto-inserting the
// space), and that decoy is a real, selectable entry in fish's completion
// menu — accepting it silently produces a nonexistent path (e.g. "demo."
// instead of "demo").
func needsNoSpace(candidates []string) bool {
	if len(candidates) != 1 {
		return true
	}
	return strings.HasSuffix(candidates[0], "/")
}
