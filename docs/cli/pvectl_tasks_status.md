## pvectl tasks status

Show the current status of a Proxmox task

```
pvectl tasks status <upid> [flags]
```

### Options

```
  -h, --help   help for status
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl tasks](pvectl_tasks.md)	 - Manage cluster tasks
