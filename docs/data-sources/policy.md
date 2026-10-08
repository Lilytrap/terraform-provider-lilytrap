---
page_title: "lilytrap_policy Data Source"
description: Which decoy locations the workspace's ignore rules exclude.
---

# lilytrap_policy

Checks resource paths against the workspace's ignore rules (set in the dashboard or `lilytrap.json`), plus any of your own.
Use it to skip decoys in places your team has excluded.
Paths use `.lilyignore` syntax, e.g. `vault/<mount>/<path>`, `aws/<account>/<region>/secretsmanager/<name>` or `k8s/<namespace>/secrets/<name>`.

## Example

```terraform
locals {
  vault_paths = { prod = "vault/secret/prod/admin", staging = "vault/secret/staging/admin" }
}

data "lilytrap_policy" "vault" {
  paths  = values(local.vault_paths)
  ignore = ["vault/secret/legacy/"]
}

# One decoy per environment the rules allow.
resource "vault_kv_secret_v2" "decoy" {
  for_each = { for env, p in local.vault_paths : env => p if contains(data.lilytrap_policy.vault.allowed, p) }
  mount    = "secret"
  name     = "${each.key}/admin"
  data_json = jsonencode({ token = lilytrap_decoy_credential.admin.secret })
}
```

## Schema

### Required

- `paths` (List of String) Resource paths about to receive decoys. 1 to 1000.

### Optional

- `ignore` (List of String) Extra `.lilyignore` patterns. The workspace's rules always apply too, and can't be undone here.

### Read-only

- `ignored` (List of String) The paths the rules exclude.
- `allowed` (List of String) The paths decoys may go to, in input order.
