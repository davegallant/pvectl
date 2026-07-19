package cmd

import (
	"github.com/davegallant/pvectl/internal/ssh"
	"github.com/spf13/cobra"
)

var ctExecCmd = &cobra.Command{
	Use:               "exec <name-or-vmid> -- <command> [args...]",
	Short:             "Run a command inside a container over SSH, non-interactively",
	Args:              cobra.MinimumNArgs(2),
	ValidArgsFunction: completeContainerNames,
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
