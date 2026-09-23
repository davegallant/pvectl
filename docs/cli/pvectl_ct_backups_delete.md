## pvectl ct backups delete

Delete one of a container's backups

```
pvectl ct backups delete <name-or-vmid> [flags]
```

### Options

```
  -h, --help           help for delete
      --volid string   backup volid to delete (skips the interactive listing/prompt when set, along with the name-or-vmid argument)
  -y, --yes            skip the confirmation prompt
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl ct backups](pvectl_ct_backups.md)	 - Manage a container's backups
