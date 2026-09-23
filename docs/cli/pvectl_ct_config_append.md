## pvectl ct config append

Append raw lxc.* config lines (e.g. cgroup rules, bind mounts) not exposed by the Proxmox API (requires SSH)

```
pvectl ct config append <name-or-vmid> [flags]
```

### Options

```
  -h, --help               help for append
      --line stringArray   raw "lxc.subkey: value" config line to append (repeatable)
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl ct config](pvectl_ct_config.md)	 - Manage a container's config
