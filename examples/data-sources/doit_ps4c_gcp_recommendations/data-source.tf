# List all PS4C GCP purchase recommendations for a billing account
data "doit_ps4c_gcp_recommendations" "all" {
  billing_account_id = "012345-6789AB-CDEF01"
}

output "recommendation_count" {
  value = data.doit_ps4c_gcp_recommendations.all.row_count
}

output "recommendations" {
  value = [for rec in data.doit_ps4c_gcp_recommendations.all.items : {
    service                = rec.service
    region                 = rec.region
    recommended_commitment = rec.recommended_commitment
    current_commitment     = rec.current_commitment
  }]
}
