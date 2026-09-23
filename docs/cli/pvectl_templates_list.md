## pvectl templates list

List LXC templates available to download (or --downloaded for those already on storage)

```
pvectl templates list [flags]
```

### Options

```
      --downloaded    list templates already present on storage instead of the download catalog
  -h, --help          help for list
      --node string   node to query (defaults to any node — the catalog is cluster-wide)
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
