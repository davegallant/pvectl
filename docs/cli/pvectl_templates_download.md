## pvectl templates download

Download an LXC OS template onto a storage

```
pvectl templates download <template> [flags]
```

### Options

```
  -h, --help             help for download
      --node string      node to run the download on (defaults to any node)
      --storage string   storage to download onto (prompts if omitted)
```

### Options inherited from parent commands

```
      --debug                   log Proxmox API request/response activity to stderr
  -o, --output string           output format: "table" or "json" (supported read commands and raw API) (default "table")
      --verbose                 show Proxmox task IDs (UPIDs) alongside action output
      --wait-timeout duration   maximum time to wait for a Proxmox task (0 waits indefinitely)
```

### SEE ALSO

* [pvectl templates](pvectl_templates.md)	 - Manage LXC OS templates
