## pvectl qm stop

Stop a VM immediately (hard power-off, no ACPI/guest involvement)

```
pvectl qm stop <name-or-vmid> [flags]
```

### Options

```
  -h, --help   help for stop
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
