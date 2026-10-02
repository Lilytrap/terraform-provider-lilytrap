---
page_title: "lilytrap_trap Data Source"
description: Where decoys point and where forwarders send events.
---

# lilytrap_trap

## Example

```terraform
data "lilytrap_trap" "this" {}
```

## Schema

### Read-only

- `trap_url` (String) Base URL decoys point at.
- `api_url` (String)
- `ingest_url` (String) Append `/aws`, `/gcp`, `/azure` or `/host`.
