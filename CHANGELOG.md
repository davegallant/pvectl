# Changelog

## Unreleased

- **BREAKING:** Removed the interactive fuzzy-picker/action-menu (`ct select`/`qm select` and the "no argument falls back to the picker" behavior everywhere it existed). `pvectl` is now a strict CLI — every `ct`/`qm` command that acts on a guest (`enter`, `edit`, `start`, `stop`, `reboot`, `backups create/list/delete/restore`, `snapshots create/list/delete/rollback`, `migrate`) now requires a `<name-or-vmid>` argument instead of accepting an optional one.
- `ct backups restore`/`qm backups restore`: restore a container or VM from a backup, either in place (from one of its own backups, always confirmed) or, with `--node`, from any backup found on a node for disaster recovery when the original guest no longer exists.
- Shell completion (`pvectl completion`) now suggests VM/container names for every `ct`/`qm` command's `<name-or-vmid>` argument, fetched live from the cluster on each Tab press.
- `pvectl ct enter`/`pvectl qm enter` gain an API-based console method (`--method api`, or set as default with `pvectl setup`) as an alternative to the default SSH path — opens Proxmox's termproxy websocket directly over the stored API token, so no SSH access to the node is required.
- `pvectl config view`: prints the on-disk config as YAML.

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
