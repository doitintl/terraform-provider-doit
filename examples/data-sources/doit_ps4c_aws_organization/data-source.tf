# Retrieve a single AWS Organization tracked by PerfectScale for Commitments (PS4C)
data "doit_ps4c_aws_organization" "example" {
  management_account_id = "123456789012"
}

output "organization_display_name" {
  value = data.doit_ps4c_aws_organization.example.display_name
}

output "organization_onboarding_status" {
  value = data.doit_ps4c_aws_organization.example.onboarding_status
}

output "organization_savings_totals" {
  value = data.doit_ps4c_aws_organization.example.savings_totals
}
