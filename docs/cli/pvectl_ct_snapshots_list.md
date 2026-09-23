## pvectl ct snapshots list

List a container's snapshots

```
pvectl ct snapshots list <name-or-vmid> [flags]
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl ct snapshots](pvectl_ct_snapshots.md)	 - Manage a container's snapshots
