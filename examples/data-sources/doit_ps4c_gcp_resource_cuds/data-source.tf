# Retrieve resource-based commitments (CUDs) for a GCP billing account
data "doit_ps4c_gcp_resource_cuds" "active" {
  billing_account_id = "012345-6789AB-CDEF01"
  status             = "ACTIVE"
}

output "cud_count" {
  value = data.doit_ps4c_gcp_resource_cuds.active.row_count
}

output "cuds" {
  value = [for cud in data.doit_ps4c_gcp_resource_cuds.active.items : {
    commitment_numeric_id = cud.commitment_numeric_id
    state                 = cud.state
    region                = cud.region
    plan                  = cud.plan
  }]
}
