# Retrieve all GCP Billing Accounts tracked by PerfectScale for Commitments (PS4C)
data "doit_ps4c_gcp_billing_accounts" "all" {}

output "billing_account_count" {
  value = data.doit_ps4c_gcp_billing_accounts.all.row_count
}

output "billing_accounts" {
  value = [for account in data.doit_ps4c_gcp_billing_accounts.all.items : {
    billing_account_id = account.billing_account_id
    display_name       = account.display_name
  }]
}
