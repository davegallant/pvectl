package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
)

// ctRenameName backs `ct rename`'s `--name` flag, which skips the
// interactive prompt — set only when the `ct rename` subcommand
// registers it, so the `ct select` menu's rename action still always
// prompts.
var ctRenameName string

// runRename renames c, prompting for the new hostname unless ctRenameName
// (the `--name` flag) is already set.
func runRename(client *api.Client, c api.Container) error {
	newName := ctRenameName
	if newName == "" {
		fmt.Print("new hostname: ")
		reader := bufio.NewReader(os.Stdin)
		nameLine, _ := reader.ReadString('\n')
		newName = strings.TrimSpace(nameLine)
	}
	if newName == "" {
		return fmt.Errorf("new hostname required")
	}
	return renameContainer(client, c, newName)
}

// renameContainer updates c's hostname config field to newName — unlike
// start/stop/snapshot/backup/migrate, this is a plain config write that
// Proxmox applies immediately, not an async task, so there's no UPID to
// poll via runProgressAction (same reasoning as applyEdit in edit.go).
// Split out from runRename so the digest-fetch/PUT/digest-mismatch logic
// is directly unit-testable without touching stdin.
func renameContainer(client *api.Client, c api.Container, newName string) error {
	ctx := context.Background()
	cfg, err := client.GetConfig(ctx, c.Node, c.VMID)
	if err != nil {
		return fmt.Errorf("fetching config: %w", err)
	}

	err = client.PutConfig(ctx, c.Node, c.VMID, map[string]string{"hostname": newName}, cfg.Digest)
	if errors.Is(err, api.ErrDigestMismatch) {
		return fmt.Errorf("config changed elsewhere, re-run rename")
	}
	if err != nil {
		return fmt.Errorf("renaming %s (%d): %w", c.Name, c.VMID, err)
	}

	fmt.Printf("renamed %s (%d) to %s\n", c.Name, c.VMID, newName)
	return nil
}

func init() {
	ctRenameCmd := newSimpleActionCmd("rename", "Rename a container", runRename)
	ctRenameCmd.Flags().StringVar(&ctRenameName, "name", "", "new hostname (skips the interactive prompt when set, along with the name-or-vmid argument)")
	ctCmd.AddCommand(ctRenameCmd)
}
