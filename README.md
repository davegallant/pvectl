# pvectl

**A command-line companion for Proxmox VE.** Manage a container
or VM - start, stop, snapshot, back up, edit, or migrate it — all
without leaving your terminal or memorizing a single vmid.

![pvectl demo](pvectl-demo.gif)

## Why pvectl

- **Unified management.** No need to remember what node your hosts are on. Run it on your laptop.
- **Tab completion.** No need to remember ct or vm ids; tab completion suggests names as you type.
- **Status polling.** Anything that runs as a background
  Proxmox task (start/stop/reboot/snapshot/backup/migrate) shows a live
  spinner and a final `✓`/`✗` summary with timing.
- **Secrets stay in your keychain.** `pvectl setup` stores your API token
  secret in the OS keychain, if available.
- **Use with your coding agent.** Works with your harness of choice.

> [!WARNING]
> **Experimental, provided as-is, with no warranty.** pvectl can start,
> stop, edit, and permanently delete things on your cluster (including
> [backups](#backups) and [snapshots](#snapshots)). Review what it's about to do before confirming.
> Tested only against Proxmox VE 9+; earlier versions may behave
> differently or not work at all.

## Install

With Homebrew (macOS):

```sh
brew install davegallant/public/pvectl
```

With curl (Linux):

```sh
curl -fsSL https://raw.githubusercontent.com/davegallant/pvectl/main/scripts/install.sh | sh
```

With Nix:

```sh
nix profile install github:davegallant/pvectl
```

Or build from source:

```sh
git clone https://github.com/davegallant/pvectl.git
cd pvectl
go build -o pvectl ./cmd/pvectl
```

## Setup

### Create an API token

In Proxmox: **Datacenter -> Permissions -> API Tokens -> Add**.

> [!IMPORTANT]
> If **Privilege Separation** is left checked when creating the token, it
> starts with **zero permissions** until you grant it an ACL. `pvectl setup`
> will still succeed — but every command will silently show no containers.

Either uncheck Privilege Separation (the token then inherits the user's
full permissions), or grant an ACL explicitly:

```sh
pveum aclmod / -token 'user@realm!tokenid' -role PVEVMAdmin,PVEAuditor,PVESDNUser,PVEDatastoreUser 
```

### Run setup

```sh
pvectl setup
```

This prompts for your Proxmox host, token ID (`user@realm!tokenid`), and
token secret; validates them against the cluster; and stores them (the
secret goes to your OS keychain, never to a plaintext file). Add
`--insecure-skip-verify` if your cluster uses a self-signed certificate.

Once setup is complete, you can run `pvectl status` to verify your cluster is healthy:

## Usage

For full usage instructions, see [`the cli docs`](docs/cli/pvectl.md).

### Backups

Backups can be created, deleted, listed, and restored with `pvectl ct backups` and `pvectl qm backups`.

> [!CAUTION]
> Proxmox has no trash/undo for a deleted backup — this is permanent. Only
> a volid that actually appeared in the listing is accepted, so a typo
> can't reach the delete API. Requires permission to remove content on the
> target storage (e.g. `Datastore.AllocateSpace`).
>
> Restoring onto a vmid that already exists **overwrites its current
> disk/config** — also permanent, and requires typing `yes` to confirm
> unless `-y`/`--yes` is given.

### Migrations

Containers and VMs can be migrated to another node in the cluster with `pvectl ct migrate` and `pvectl qm migrate`.

> [!NOTE]
> A running container is restarted on the target node (true live migration
> of a running container isn't reliably available); a running VM is live
> migrated with no downtime.

### Snapshots

Snapshots can be created, listed, deleted, and rolled back with `pvectl ct snapshots` and `pvectl qm snapshots`.

> [!CAUTION]
> Rolling back discards every change made since the snapshot was taken —
> this cannot be undone.

### Renaming

Containers and VMs can be renamed with `pvectl ct rename` and `pvectl qm rename`.

### Creating containers

New LXC containers can be created with `pvectl ct create`.

> [!NOTE]
> There's no VM creation yet — this is LXC-only for now.

### Console access

`pvectl ct enter` and `pvectl qm enter` reach a guest's console one of two ways:

- **`ssh` (default)** — shells out to `ssh <node> pct enter <vmid>` / `ssh
  <node> qm terminal <vmid>`, so it relies on your own SSH config/agent
  rather than credentials stored by `pvectl`.
- **`api`** — opens the proxmox console websocket (the same one the web
  UI's "Console" button uses) directly over your stored API token, no SSH
  access to the node required. Enable it by answering yes to the
  console-access prompt in `pvectl setup`, or use it for a single run with
  `--method api` (or force `--method ssh` even if `api` is your configured
  default).

If using `ssh`, set up your SSH config in this format:

```
Host <node1-name>
  HostName <node1-host-or-ip>
  User <user>
  IdentityFile <path/to/key>
Host <node2-name>
  HostName <node2-host-or-ip>
  User <user>
  IdentityFile <path/to/key>
```

A VM console (either method) requires a serial console device — without
one you'll see `unable to find a serial interface`. To add one:

```sh
qm set <vmid> --serial0 socket
```

Then reboot the VM and make sure the guest OS redirects its console to it
(Linux: `console=ttyS0` on the kernel command line — most cloud images
already set this; Windows needs EMS/COM port configuration instead).

> [!NOTE]
> **LXC login prompt on `api`:** SSH's `pct enter` gives a trusted root
> shell with no login. The `api` method instead attaches to the
> container's actual console tty — like a physical console — which may
> show a login prompt. Many templates ship with root's password locked;
> set one first if you plan to rely on `api` for containers
> (`pct exec <vmid> -- passwd root`).
>
> **Detaching:** type `~.` at the start of a line to
> disconnect without ending the remote session 

For one-off non-interactive commands (`ls`, `cat`, `grep`, ...),
use `pvectl ct exec <name-or-vmid> -- <command...>`. Tab completion for the
command's own arguments SSHes into the container and lists matching remote
paths (e.g. `pvectl ct exec <name-or-vmid> -- cat docker-comp<TAB>`).

### Raw config passthrough

Raw `lxc.*` config lines (cgroup device rules, bind mounts, and anything
else with no dedicated Proxmox API parameter) can be appended to a
container's config with `pvectl ct config append --line "..."` (repeatable):

```sh
pvectl ct config append <name-or-vmid> \
  --line "lxc.cgroup2.devices.allow: c 10:200 rwm" \
  --line "lxc.mount.entry: /dev/net dev/net none bind,create=dir"
```

> [!NOTE]
> Proxmox's REST API doesn't expose raw `lxc.*` directives at all, so this
> falls back to `ssh <node> cat >> /etc/pve/lxc/<vmid>.conf`, the same way
> console access relies on your own SSH config/agent rather than
> credentials stored by `pvectl`. Restart the container for changes to
> take effect.

### Cluster tasks

The cluster's recent tasks can be listed with `pvectl tasks`, with a live-refreshing view available.

## License

pvectl is released under the [GPL-3.0](LICENSE) license.
