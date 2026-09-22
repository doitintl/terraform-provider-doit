# Retrieve the compute commitment recommendation for a GCP billing account
data "doit_ps4c_gcp_recommendation" "compute" {
  billing_account_id = "012345-6789AB-CDEF01"
  gcp_service        = "compute"
}

output "compute_recommended_commitment" {
  value = try(data.doit_ps4c_gcp_recommendation.compute.recommendation.recommended_commitment, null)
}

output "compute_eligible_usage_points" {
  value = length(data.doit_ps4c_gcp_recommendation.compute.eligible_usage)
}
