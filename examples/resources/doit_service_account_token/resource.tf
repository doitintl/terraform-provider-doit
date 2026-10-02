terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "~> 1.0"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.13"
    }
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
  }
}

resource "doit_service_account" "ci_cd" {
  name = "ci-cd-pipeline"
}

# Create an API token with the server-assigned default expiry
resource "doit_service_account_token" "default_expiry" {
  service_account_id = doit_service_account.ci_cd.id
  name               = "default-expiry"
}

# Create an API token that expires at a fixed time.
# expires_time must be an RFC 3339 timestamp in UTC with whole seconds.
# Changing name or expires_time replaces the token and mints a new credential.
resource "doit_service_account_token" "fixed_expiry" {
  service_account_id = doit_service_account.ci_cd.id
  name               = "fixed-expiry"
  expires_time       = "2099-01-01T00:00:00Z"
}

# Create a token and disable it without deleting it
resource "doit_service_account_token" "disabled" {
  service_account_id = doit_service_account.ci_cd.id
  name               = "disabled"
  state              = "disabled"
}

# The access token is only available when Terraform creates the token.
output "ci_cd_access_token" {
  value     = doit_service_account_token.default_expiry.access_token
  sensitive = true
}

# ─────────────────────────────────────────────────────────────────────────────
# Automatic rotation
# ─────────────────────────────────────────────────────────────────────────────
# time_rotating drives the token name and expiry. When the rotation date
# passes, the next apply mints a new token before deleting the old one.
# The name includes the rotation timestamp because names must be unique among
# the service account's tokens while both exist. The expiry is one week past
# the rotation date, so the token stays valid if the next apply runs late.
resource "time_rotating" "ci_token" {
  rotation_days = 90
}

resource "doit_service_account_token" "rotating" {
  service_account_id = doit_service_account.ci_cd.id
  name               = "ci-${formatdate("YYYYMMDD-hhmmss", time_rotating.ci_token.rfc3339)}"
  expires_time       = timeadd(time_rotating.ci_token.rotation_rfc3339, "168h")

  lifecycle {
    create_before_destroy = true
  }
}

# Hand the new token to CI in the same apply, for example as a GitHub Actions
# secret. The token can then authenticate the DoiT provider in CI through
# DOIT_API_TOKEN, so pipelines no longer run with a person's API key.
resource "github_actions_secret" "doit_api_token" {
  repository      = "my-finops-repo"
  secret_name     = "DOIT_API_TOKEN"
  plaintext_value = doit_service_account_token.rotating.access_token
}
