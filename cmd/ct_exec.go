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
	Short:             "Run a command inside a container over SSH, non-interactively",
	Args:              cobra.MinimumNArgs(2),
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

	return execCandidates(entries, dir, prefix), cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
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
