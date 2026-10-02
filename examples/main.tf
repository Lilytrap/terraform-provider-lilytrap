# A decoy admin token in AWS Secrets Manager, registered with Lilytrap.
# Using the token is detected by the trap. Reading the secret is detected through CloudTrail
# when you also deploy the forwarder (see the lilytrap/lilytrap Terraform modules).

terraform {
  required_providers {
    lilytrap = { source = "lilytrap/lilytrap" }
    aws      = { source = "hashicorp/aws" }
  }
}

provider "lilytrap" {} # LILYTRAP_API_KEY=wsk_...

data "aws_caller_identity" "current" {}

resource "lilytrap_decoy_credential" "admin" {}

resource "aws_secretsmanager_secret" "decoy" {
  name                    = "prod/platform/admin-api"
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "decoy" {
  secret_id = aws_secretsmanager_secret.decoy.id
  secret_string = jsonencode({
    admin_api_url = lilytrap_decoy_credential.admin.url
    admin_token   = lilytrap_decoy_credential.admin.secret
  })
}

resource "lilytrap_deployment" "prod" {
  name     = "aws/prod"
  location = "aws:${data.aws_caller_identity.current.account_id}"
  # The role running Terraform reads the secret on every plan.
  trusted_identities = [data.aws_caller_identity.current.arn]
  decoys = [{
    kit       = "cloud-secret"
    secret    = lilytrap_decoy_credential.admin.secret
    path      = "${lilytrap_decoy_credential.admin.path}/clusters"
    resources = [aws_secretsmanager_secret.decoy.arn, "secretsmanager:${aws_secretsmanager_secret.decoy.name}"]
    locations = ["aws secretsmanager ${aws_secretsmanager_secret.decoy.name}"]
  }]
}
