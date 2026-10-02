---
page_title: "Lilytrap Provider"
description: Plant decoy credentials in your infrastructure. Get alerted when an agent finds or uses them.
---

# Lilytrap Provider

Lilytrap plants decoy credentials where agents and intruders look.
Using a decoy is detected by the trap.
Reading one is detected through your cloud's audit log.

Only sha256 hashes of decoy secrets are sent to Lilytrap.

## Example

```terraform
provider "lilytrap" {} # LILYTRAP_API_KEY=wsk_...

resource "lilytrap_decoy_credential" "admin" {}

resource "lilytrap_deployment" "prod" {
  name = "aws/prod"
  decoys = [{
    kit    = "cloud-secret"
    secret = lilytrap_decoy_credential.admin.secret
    path   = "${lilytrap_decoy_credential.admin.path}/clusters"
  }]
}
```

## Schema

- `api_key` (String, Sensitive) Workspace API key (`wsk_...`). Defaults to `LILYTRAP_API_KEY`.
- `api_url` (String) Defaults to `LILYTRAP_API_URL`, then `https://api.lilytrap.com`.
