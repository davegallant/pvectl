# Changelog

## Unreleased

- Shell completion (`pvectl completion`) now suggests VM/container names for every `ct`/`qm` command's `[name-or-vmid]` argument, fetched live from the cluster on each Tab press.

## 0.1.0

- Initial release. `pvectl` talks to the Proxmox VE REST API from any machine (not just the Proxmox host) to fuzzy-find and manage LXC containers (`ct`) and QEMU VMs (`qm`).
- `ct`/`qm` actions: `start`, `stop`, `reboot`, `enter`, `migrate`, `rename`, `delete` (`-f/--force`, `--purge`, `-y/--yes`). Every action takes an optional `[name-or-vmid]` argument that skips the interactive picker.
- `ct create`: provision a new LXC container from a template, with flags for hostname, node, storage, disk/memory/swap/cores, network, SSH key/password, and `--start`; prompts interactively for anything not passed as a flag.
- `config edit`: round-trip a container's/VM's config in `$EDITOR` 
- `snapshots create`/`list`/`delete`/`rollback` and `backups create`/`list`/`delete`.
- Read-only cluster commands: `status` (with `--watch`), `nodes`, `storage`, `tasks`.
- Interactive fuzzy picker + action menu (searchable, full-screen) backed by the Proxmox live config preview.
- Live spinner progress for async Proxmox tasks, polling to completion in both interactive and scripted (non-TTY) use; failed tasks exit non-zero.
- Secrets stored in the OS keychain, with a file-based fallback; non-secret config in `~/.config/pvectl/config.yaml`.
- `--debug` API request/response logging (never logs the token).
