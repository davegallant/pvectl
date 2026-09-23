## pvectl schema

Print pvectl's command tree (names, flags, descriptions) as JSON, for agent introspection

```
pvectl schema [flags]
```

### Options

```
  -h, --help   help for schema
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
