## pvectl iso list

List ISO images already on storage

```
pvectl iso list [flags]
```

### Options

```
  -h, --help          help for list
      --node string   node whose storages to list (defaults to any node)
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl iso](pvectl_iso.md)	 - Manage ISO images
