## pvectl ct exec

Run a command inside a container, non-interactively (requires SSH)

```
pvectl ct exec <name-or-vmid> -- <command> [args...] [flags]
```

### Options

```
  -h, --help   help for exec
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl ct](pvectl_ct.md)	 - Manage containers
