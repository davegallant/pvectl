package cmd

import (
	"errors"

	"github.com/davegallant/pvectl/internal/ssh"
	"github.com/davegallant/pvectl/internal/tui"
	"github.com/spf13/cobra"
)

var qmEnterCmd = &cobra.Command{
	Use:   "enter [name-or-vmid]",
	Short: "Attach to a VM's serial console over SSH",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		v, err := resolveVM(client, args)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				return nil
			}
			return err
		}
		return ssh.EnterVM(v.Node, v.VMID)
	},
}

func init() {
	qmCmd.AddCommand(qmEnterCmd)
}
