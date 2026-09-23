## pvectl tasks

Manage cluster tasks

### Options

```
  -h, --help   help for tasks
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl](pvectl.md)	 - CLI for Proxmox VE
* [pvectl tasks list](pvectl_tasks_list.md)	 - List recent cluster tasks
* [pvectl tasks logs](pvectl_tasks_logs.md)	 - Print a Proxmox task's complete log
* [pvectl tasks status](pvectl_tasks_status.md)	 - Show the current status of a Proxmox task
* [pvectl tasks wait](pvectl_tasks_wait.md)	 - Wait for a Proxmox task and report its outcome
