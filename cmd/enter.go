package cmd

import (
	"errors"

	"github.com/davegallant/pvectl/internal/ssh"
	"github.com/davegallant/pvectl/internal/tui"
	"github.com/spf13/cobra"
)

var enterCmd = &cobra.Command{
	Use:               "enter [name-or-vmid]",
	Short:             "Enter a container's shell over SSH",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeContainerNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		c, err := resolveContainer(client, args)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				return nil
			}
			return err
		}
		return ssh.Enter(c.Node, c.VMID)
	},
}

func init() {
	ctCmd.AddCommand(enterCmd)
}
