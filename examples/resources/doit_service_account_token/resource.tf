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
