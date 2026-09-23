## pvectl api put

PUT to a raw Proxmox API path

```
pvectl api put <path> [flags]
```

### Options

```
      --data stringArray   a key=value parameter to send (repeatable)
  -h, --help               help for put
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl api](pvectl_api.md)	 - Make a raw Proxmox API call — an escape hatch for endpoints pvectl has no dedicated command for
