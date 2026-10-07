# Retrieve spend-based commitments (CUDs) for a GCP billing account
data "doit_ps4c_gcp_spend_cuds" "active" {
  billing_account_id = "012345-6789AB-CDEF01"
  status             = "active"
}

output "cud_count" {
  value = data.doit_ps4c_gcp_spend_cuds.active.row_count
}

output "cuds" {
  value = [for cud in data.doit_ps4c_gcp_spend_cuds.active.items : {
    cud_id            = cud.cud_id
    cud_product_name  = cud.cud_product_name
    state             = cud.state
    commitment_amount = cud.commitment_amount
    region            = cud.region
    term              = cud.term
  }]
}
