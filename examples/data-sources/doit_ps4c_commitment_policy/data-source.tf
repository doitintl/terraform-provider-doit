# Retrieve details for a specific PS4C commitment policy
data "doit_ps4c_commitment_policy" "balanced" {
  policy_id = "balanced"
}

output "policy_target_coverage" {
  value = data.doit_ps4c_commitment_policy.balanced.target_coverage
}

output "policy_assignments" {
  value = data.doit_ps4c_commitment_policy.balanced.assignments
}
