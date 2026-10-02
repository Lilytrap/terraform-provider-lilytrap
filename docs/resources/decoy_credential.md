---
page_title: "lilytrap_decoy_credential Resource"
description: A decoy secret and the trap URL it unlocks.
---

# lilytrap_decoy_credential

A decoy secret in Lilytrap's own format, and the trap URL it claims to unlock.
It's generated locally and kept in Terraform state.
Register it with `lilytrap_deployment`.

## Example

```terraform
resource "lilytrap_decoy_credential" "admin" {
  prefix  = "svc_"
  keepers = { rotation = "2026-q4" }
}
```

## Schema

### Optional

- `prefix` (String) Defaults to `adm_`. Prefixes of real providers' keys are refused.
- `path_prefix` (String) Defaults to `/internal/platform-admin`. A random segment is appended.
- `keepers` (Map of String) Change any value to rotate.

### Read-only

- `secret` (String, Sensitive)
- `path` (String) Path on the trap.
- `url` (String) `trap_url` + `path`.
- `trap_url` (String)
