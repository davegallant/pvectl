## pvectl qm config unset

Unset a regular config field with digest protection

```
pvectl qm config unset <name-or-vmid> <field> [flags]
```

### Options

```
  -h, --help   help for unset
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl qm config](pvectl_qm_config.md)	 - Manage a VM's config
