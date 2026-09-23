## pvectl api

Make a raw Proxmox API call — an escape hatch for endpoints pvectl has no dedicated command for

### Options

```
  -h, --help   help for api
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
* [pvectl api delete](pvectl_api_delete.md)	 - DELETE a raw Proxmox API path
* [pvectl api get](pvectl_api_get.md)	 - GET a raw Proxmox API path (e.g. /nodes, /cluster/status)
* [pvectl api post](pvectl_api_post.md)	 - POST to a raw Proxmox API path
* [pvectl api put](pvectl_api_put.md)	 - PUT to a raw Proxmox API path
