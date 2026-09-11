# Retrieve a single GCP Billing Account tracked by PerfectScale for Commitments (PS4C)
data "doit_ps4c_gcp_billing_account" "example" {
  billing_account_id = "012345-6789AB-CDEF01"
}

output "billing_account_display_name" {
  value = data.doit_ps4c_gcp_billing_account.example.display_name
}

output "billing_account_currency" {
  value = data.doit_ps4c_gcp_billing_account.example.currency
}

output "billing_account_onboarding_status" {
  value = data.doit_ps4c_gcp_billing_account.example.onboarding_status
}

output "billing_account_savings_totals" {
  value = data.doit_ps4c_gcp_billing_account.example.savings_totals
}
