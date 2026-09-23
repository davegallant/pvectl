## pvectl nodes reboot

Reboot a Proxmox node

```
pvectl nodes reboot <node> [flags]
```

### Options

```
  -h, --help   help for reboot
  -y, --yes    skip the confirmation prompt
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl nodes](pvectl_nodes.md)	 - Manage Proxmox cluster nodes
