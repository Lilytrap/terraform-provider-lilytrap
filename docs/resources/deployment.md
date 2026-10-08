---
page_title: "lilytrap_deployment Resource"
description: Registers decoys with Lilytrap and retires them on destroy.
---

# lilytrap_deployment

Registers decoys with Lilytrap by hash.
Any change creates a new deployment.
Destroying it retires the decoys, so they stop matching.
Every refresh tells Lilytrap the deployment is still there, so it isn't reported as stale.

Set `paths` to where the decoys live, and the plan fails if the workspace's ignore rules (or `ignore`) exclude any of them, before anything is created.
To skip excluded locations instead of failing, filter them first with the `lilytrap_policy` data source.

## Example

```terraform
resource "lilytrap_deployment" "prod" {
  name               = "aws/prod"
  location           = "aws:111122223333/us-east-2"
  paths              = ["aws/111122223333/us-east-2/secretsmanager/prod/platform/admin-api"]
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
- `paths` (List of String) Where the decoys live, as `.lilyignore` resource paths. Checked against ignore rules at plan time.
- `ignore` (List of String) Extra `.lilyignore` patterns for `paths`, on top of the workspace's rules.

### Read-only

- `id` (String) Deployment id.
- `ingest_key` (String, Sensitive) Key for the forwarder.
- `ingest_url` (String) Append `/aws`, `/gcp`, `/azure` or `/host`.
- `trap_url` (String)
- `created_at` (String)
