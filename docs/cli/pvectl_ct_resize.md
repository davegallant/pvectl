## pvectl ct resize

Grow a container disk (cannot shrink)

```
pvectl ct resize <name-or-vmid> [flags]
```

### Options

```
      --disk string   disk to resize (e.g. "rootfs", "mp0") (default "rootfs")
  -h, --help          help for resize
      --size string   new size: "+2G" to grow by 2GB, or "10G" to set the total size (required)
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
