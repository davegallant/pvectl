# pvectl

**A command-line companion for Proxmox VE.** Manage a container
or VM - start, stop, snapshot, back up, edit, or migrate it — all
without leaving your terminal or memorizing a single vmid.

![pvectl demo](pvectl-demo.gif)

> [!WARNING]
> **Experimental, provided as-is, with no warranty.** pvectl can start,
> stop, edit, and permanently delete things on your cluster (including
> [backups](#backups) and [snapshots](#snapshots)). Review what it's about to do before confirming.
> Tested only against Proxmox VE 9+; earlier versions may behave
> differently or not work at all.

## Why pvectl

- **Unified management.** You don't have to remember what node your hosts are on. Run it on your laptop.
- **Status polling.** Anything that runs as a background
  Proxmox task (start/stop/reboot/snapshot/backup/migrate) shows a live
  spinner and a final `✓`/`✗` summary with timing.
- **Secrets stay in your keychain.** `pvectl setup` stores your API token
  secret in the OS keychain, if available.
- **Use with your coding agent.** Works well with your coding agent of choice.
- **Fuzzy finder, not a vmid lookup table.** `pvectl ct select`/`pvectl qm select` let you
  type a few letters of a name instead of remembering `101` vs `102`.

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

Backups can be created, deleted, and listed with `pvectl ct backups` and `pvectl qm backups`.

> [!CAUTION]
> Proxmox has no trash/undo for a deleted backup — this is permanent. Only
> a volid that actually appeared in the listing is accepted, so a typo
> can't reach the delete API. Requires permission to remove content on the
> target storage (e.g. `Datastore.AllocateSpace`).

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

Console access requires `ssh` access to the node, so it relies on
your own SSH config/agent rather than credentials stored by `pvectl`.

Setup your SSH config in this format:

```
Host <node1>
  HostName <node1>
  User <user>
  IdentityFile <path/to/key>
Host <node2>
  HostName <node2>
  User <user>
  IdentityFile <path/to/key>
```

`pvectl ct enter` falls back to `ssh <node> pct enter <vmid>`, so it relies on
your own SSH config/agent rather than credentials stored by `pvectl`.

`pvectl qm enter` falls back to `ssh <node> qm terminal <vmid>`, which requires
the VM to have a serial console device — without one you'll see `unable
to find a serial interface`. To add one:

```sh
qm set <vmid> --serial0 socket
```

Then reboot the VM and make sure the guest OS redirects its console to it
(Linux: `console=ttyS0` on the kernel command line — most cloud images
already set this; Windows needs EMS/COM port configuration instead).

> [!NOTE]
> `qm terminal` is a raw serial passthrough, not a shell. Press **Ctrl-O** to detach 

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
