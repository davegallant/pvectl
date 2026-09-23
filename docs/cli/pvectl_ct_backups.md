## pvectl ct backups

Manage a container's backups

### Options

```
  -h, --help   help for backups
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
* [pvectl ct backups create](pvectl_ct_backups_create.md)	 - Create a backup
* [pvectl ct backups delete](pvectl_ct_backups_delete.md)	 - Delete one of a container's backups
* [pvectl ct backups list](pvectl_ct_backups_list.md)	 - List a container's backups
* [pvectl ct backups restore](pvectl_ct_backups_restore.md)	 - Restore a container from a backup
