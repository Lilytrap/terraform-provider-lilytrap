---
page_title: "lilytrap_deployment Resource"
description: Registers decoys with Lilytrap and retires them on destroy.
---

# lilytrap_deployment

Registers decoys with Lilytrap by hash.
Any change creates a new deployment.
Destroying it retires the decoys, so they stop matching.

## Example

```terraform
resource "lilytrap_deployment" "prod" {
  name               = "aws/prod"
  location           = "aws:111122223333/us-east-2"
  trusted_identities = ["arn:aws:iam::111122223333:role/terraform"]
  decoys = [{
    kit       = "cloud-secret"
    secret    = lilytrap_decoy_credential.admin.secret
    path      = "${lilytrap_decoy_credential.admin.path}/clusters"
    resources = ["secretsmanager:prod/platform/admin-api"]
  }]
}
```

## Schema

### Required

- `name` (String) Shown in the dashboard.
- `decoys` (Attributes List) 1 to 500 decoys:
  - `kit` (String, required) What the decoy pretends to be, e.g. `cloud-secret`.
  - `secret` (String, required, Sensitive) Only its hash leaves Terraform.
  - `path` (String, required) Path on the trap the secret is used against.
  - `method` (String) Defaults to `GET`.
  - `kind` (String) `bearer` (default), `header-key`, `basic-auth` or `signed-url`.
  - `hop` (Number) Position in the breadcrumb trail. Defaults to 1.
  - `resources` (List of String) Audit-log identifiers whose reads mean the decoy was found.
  - `locations` (List of String) Where it's planted, for the incident brief.

### Optional

- `target` (String) `cloud` (default), `k8s` or `host`.
- `location` (String) e.g. `aws:111122223333/us-east-2`.
- `trusted_identities` (List of String) Reads by these never alert. IP, CIDR, ARN, glob or `exe:/path`.
- `access_detection` (Boolean) Defaults to true. Generates `ingest_key` for an audit-log forwarder.

### Read-only

- `id` (String) Deployment id.
- `ingest_key` (String, Sensitive) Key for the forwarder.
- `ingest_url` (String) Append `/aws`, `/gcp`, `/azure` or `/host`.
- `trap_url` (String)
- `created_at` (String)
