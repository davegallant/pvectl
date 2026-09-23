## pvectl qm config

Manage a VM's config

### Options

```
  -h, --help   help for config
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl qm](pvectl_qm.md)	 - Manage QEMU VMs
* [pvectl qm config edit](pvectl_qm_config_edit.md)	 - Edit a VM's config in $EDITOR
* [pvectl qm config set](pvectl_qm_config_set.md)	 - Set a regular config field with digest protection
* [pvectl qm config unset](pvectl_qm_config_unset.md)	 - Unset a regular config field with digest protection
* [pvectl qm config view](pvectl_qm_config_view.md)	 - Show a VM's config
