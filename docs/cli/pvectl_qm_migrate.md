## pvectl qm migrate

Migrate a VM to another node

```
pvectl qm migrate <name-or-vmid> [flags]
```

### Options

```
  -h, --help                    help for migrate
      --target string           node to migrate to (skips the interactive prompt when set)
      --target-storage string   storage on the target node for the guest's volumes, e.g. local-lvm or a source:target,... mapping (default: the same storage ID as on the source node)
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl qm](pvectl_qm.md)	 - Manage QEMU VMs
