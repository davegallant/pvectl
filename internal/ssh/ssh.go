package ssh

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// buildCmd constructs the `ssh -t <node> pct enter <vmid>` command, wired
// to the current process's stdio for a real interactive session. -t forces
// pseudo-terminal allocation: ssh does not allocate one by default when a
// remote command is given, and pct enter needs a real pty to attach the
// container's console to — without it, pct enter hangs indefinitely.
func buildCmd(node string, vmid int) *exec.Cmd {
	cmd := exec.Command("ssh", "-t", node, "pct", "enter", fmt.Sprintf("%d", vmid))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// Enter execs into the given container over SSH, replacing this process's
// stdio for an interactive shell. It returns the underlying command's exit
// error if it doesn't exit 0 — callers should let that propagate to main's
// exit code, not wrap it.
func Enter(node string, vmid int) error {
	return buildCmd(node, vmid).Run()
}

// buildVMCmd constructs the `ssh -t <node> qm terminal <vmid>` command,
// wired to the current process's stdio for a real interactive session.
// Unlike buildCmd (LXC's `pct enter`, which always works), `qm terminal`
// only works if the VM has a serial console device configured — otherwise
// it errors, and that error is propagated as-is by EnterVM (see EnterVM's
// doc comment).
func buildVMCmd(node string, vmid int) *exec.Cmd {
	cmd := exec.Command("ssh", "-t", node, "qm", "terminal", fmt.Sprintf("%d", vmid))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// EnterVM attaches to the given VM's serial console over SSH, replacing
// this process's stdio for an interactive session. It returns the
// underlying command's exit error if it doesn't exit 0 — callers should
// let that propagate to main's exit code, not wrap it. If the VM has no
// serial console configured, `qm terminal`'s own error surfaces as-is;
// this is not specially detected or given a custom hint, matching Enter's
// existing "let SSH's own error output speak for itself" policy.
//
// Unlike a normal shell (which you exit with `exit`/Ctrl-D), `qm terminal`
// is a raw serial passthrough with its own detach key — Ctrl-C and other
// signals go straight to the VM's console instead of the local ssh/qm
// process, so printing the real escape sequence up front is the difference
// between a clean exit and an apparent hang.
func EnterVM(node string, vmid int) error {
	fmt.Println("Attaching to VM console — press Ctrl-O to exit.")
	return buildVMCmd(node, vmid).Run()
}

// buildAppendRawConfigCmd constructs `ssh <node> cat >> /etc/pve/lxc/<vmid>.conf`,
// with lines written to the remote command's stdin rather than embedded in
// the command string — avoids any need to shell-quote arbitrary raw lxc.*
// config text (bind-mount paths, cgroup rules) on the remote end. vmid comes
// from an already-resolved container, so the path is safe to build directly.
func buildAppendRawConfigCmd(node string, vmid int, lines []string) *exec.Cmd {
	path := fmt.Sprintf("/etc/pve/lxc/%d.conf", vmid)
	cmd := exec.Command("ssh", node, "cat", ">>", path)
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	return cmd
}

// AppendRawConfig appends lines (each a raw "lxc.subkey: value" passthrough
// config line, e.g. cgroup device rules or bind mounts) to node's
// /etc/pve/lxc/<vmid>.conf over SSH. This exists because Proxmox's REST API
// does not expose raw lxc.* directives at all — confirmed by Proxmox
// maintainers on the community forum, not a pvectl gap — so PutConfig has
// no parameter for them and a direct file write on the node is the only way
// to set them, same as Enter/EnterVM are the only way to get an interactive
// console.
func AppendRawConfig(node string, vmid int, lines []string) error {
	return runCapturingStderr(buildAppendRawConfigCmd(node, vmid, lines))
}

// runCapturingStderr runs cmd and, on failure, wraps the exit error with
// whatever the remote side wrote to stderr — shared by AppendRawConfig/
// Unlock/UnlockVM, the SSH calls whose failure text (e.g. Proxmox's own
// "unable to unlock, not locked" message) the user actually needs to see,
// not just ssh's own bare exit-code error.
func runCapturingStderr(cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

// buildUnlockCmd constructs `ssh <node> pct unlock <vmid>`. There is no
// REST API equivalent: `pct unlock`/`qm unlock` run entirely on the node
// (removing the container's/VM's "lock" config entry directly via
// PVE::LXC::Config/PVE::QemuConfig, as root) rather than calling into
// pvedaemon — confirmed the hard way against a real cluster, where PUT
// .../config rejects a lock removal outright ("CT is locked (backup)"),
// and adding Proxmox's own `skiplock` parameter (used by action endpoints
// like start/stop) is itself rejected by the config endpoint's schema, so
// it isn't accepted there either. SSH is the only way in, same rationale
// as AppendRawConfig.
func buildUnlockCmd(node string, vmid int) *exec.Cmd {
	return exec.Command("ssh", node, "pct", "unlock", fmt.Sprintf("%d", vmid))
}

// Unlock clears vmid's lock on node over SSH, matching `pct unlock`.
func Unlock(node string, vmid int) error {
	return runCapturingStderr(buildUnlockCmd(node, vmid))
}

// buildUnlockVMCmd is buildUnlockCmd's mirror for QEMU VMs.
func buildUnlockVMCmd(node string, vmid int) *exec.Cmd {
	return exec.Command("ssh", node, "qm", "unlock", fmt.Sprintf("%d", vmid))
}

// UnlockVM clears vmid's lock on node over SSH, matching `qm unlock`.
func UnlockVM(node string, vmid int) error {
	return runCapturingStderr(buildUnlockVMCmd(node, vmid))
}

// buildExecCmd constructs the `ssh <node> pct exec <vmid> -- <command...>`
// command, wired to the current process's stdio. Unlike buildCmd (`pct
// enter`), no `-t` is passed: pct exec doesn't need a pty to run a command
// and stream its output, and forcing one would corrupt clean output for
// scripting/piping.
func buildExecCmd(node string, vmid int, command []string) *exec.Cmd {
	args := append([]string{node, "pct", "exec", fmt.Sprintf("%d", vmid), "--"}, command...)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// Exec runs command inside the given container over SSH, non-interactively,
// with the current process's stdio wired straight through. It returns the
// underlying command's exit error as-is (same policy as Enter/EnterVM) —
// callers should let that propagate to main's exit code, not wrap it. A
// guest command that exits non-zero (grep, test, false, ...) is expected,
// not a pvectl failure.
func Exec(node string, vmid int, command []string) error {
	return buildExecCmd(node, vmid, command).Run()
}

// buildListDirCmd constructs the `ssh -o BatchMode=yes <node> pct exec
// <vmid> -- ls -1p -- <dir>` command used to drive remote path completion
// for `ct exec`'s trailing arguments — the same "SSH out and list the
// remote directory" technique bash-completion's _scp/_rsync functions use
// for remote path completion. BatchMode=yes disables interactive prompts
// (password/host-key confirmation), so a stalled auth attempt fails fast
// instead of hanging the shell's Tab key; -p appends a trailing "/" to
// directory entries so completions can preserve that distinction locally.
func buildListDirCmd(ctx context.Context, node string, vmid int, dir string) *exec.Cmd {
	return exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", node, "pct", "exec", fmt.Sprintf("%d", vmid), "--", "ls", "-1p", "--", dir)
}

// parseListDirOutput splits ls -1p's stdout into entries, dropping the
// trailing empty line left by ls's final newline. Split out from ListDir
// so the parsing is unit-testable without a live SSH connection.
func parseListDirOutput(out []byte) []string {
	trimmed := strings.TrimRight(string(out), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// ListDir lists dir's immediate entries inside the given container over
// SSH, for remote path completion. ctx should carry a short deadline —
// completion must never hang the shell waiting on a stalled or
// unreachable node. Any failure (timeout, container stopped, dir missing,
// permission denied) returns nil: completion has no useful way to surface
// an error to the shell, so a failed listing degrades to "no completions"
// rather than propagating.
func ListDir(ctx context.Context, node string, vmid int, dir string) []string {
	out, err := buildListDirCmd(ctx, node, vmid, dir).Output()
	if err != nil {
		return nil
	}
	return parseListDirOutput(out)
}
