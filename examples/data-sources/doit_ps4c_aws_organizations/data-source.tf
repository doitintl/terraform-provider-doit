# Retrieve all AWS Organizations tracked by PerfectScale for Commitments (PS4C)
data "doit_ps4c_aws_organizations" "all" {}

output "organization_count" {
  value = data.doit_ps4c_aws_organizations.all.row_count
}

output "organizations" {
  value = [for org in data.doit_ps4c_aws_organizations.all.items : {
    management_account_id = org.management_account_id
    display_name           = org.display_name
  }]
}
