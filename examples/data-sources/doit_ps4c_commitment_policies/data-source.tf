# List all PS4C commitment policies available to the tenant
data "doit_ps4c_commitment_policies" "all" {}

output "policy_count" {
  value = data.doit_ps4c_commitment_policies.all.row_count
}

output "policies" {
  value = data.doit_ps4c_commitment_policies.all.items
}
