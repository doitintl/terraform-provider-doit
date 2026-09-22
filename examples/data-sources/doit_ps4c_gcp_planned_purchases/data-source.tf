# Retrieve planned commitment purchase projections for a GCP billing account
data "doit_ps4c_gcp_planned_purchases" "example" {
  billing_account_id = "012345-6789AB-CDEF01"
  gcp_service        = "compute"
}

output "purchase_groups_count" {
  value = data.doit_ps4c_gcp_planned_purchases.example.row_count
}

output "planned_purchases" {
  value = [for item in data.doit_ps4c_gcp_planned_purchases.example.items : {
    service = item.service
    regions = [for r in item.regions : {
      region                   = r.region
      status                   = r.status
      purchase_approval_status = r.purchase_approval_status
      final_commitment         = r.final_commitment
      steps_count              = length(r.steps)
    }]
  }]
}
