data "doit_billing_gcp_requirements" "this" {}

# The roles to grant to the DoiT service account before connecting
output "billing_grants" {
  value = data.doit_billing_gcp_requirements.this.grants
}

output "doit_service_account" {
  value = data.doit_billing_gcp_requirements.this.service_account_email
}
