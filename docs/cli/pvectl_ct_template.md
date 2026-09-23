## pvectl ct template

Convert a container to a template (irreversible)

```
pvectl ct template <name-or-vmid> [flags]
```

### Options

```
  -h, --help   help for template
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

* [pvectl ct](pvectl_ct.md)	 - Manage containers
