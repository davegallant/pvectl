package cmd

import (
	"context"
	"fmt"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/davegallant/pvectl/internal/term"
)

// enterConsole reaches a guest's console using methodOverride if set (a
// per-invocation --method flag), otherwise whichever method the stored
// config selects. "api" opens Proxmox's termproxy websocket directly
// (internal/term); "ssh" (the default) shells out to
// `ssh node pct enter`/`qm terminal` via sshEnter — passed in since that
// call differs between LXC and QEMU (internal/ssh.Enter vs EnterVM).
func enterConsole(client *api.Client, node string, vmid int, kind api.ConsoleKind, sshEnter func(node string, vmid int) error, methodOverride string) error {
	method := methodOverride
	switch method {
	case "":
		var err error
		method, err = consoleMethod()
		if err != nil {
			return friendlySetupError(err)
		}
	case "ssh", "api":
		// valid override, use as-is
	default:
		return fmt.Errorf(`invalid --method %q: must be "ssh" or "api"`, method)
	}

	if method != "api" {
		return sshEnter(node, vmid)
	}

	conn, err := client.OpenConsole(context.Background(), node, vmid, kind)
	if err != nil {
		return err
	}
	return term.Attach(context.Background(), conn)
}
