resource "doit_billing_gcp_account" "this" {
  billing_account_id = "012345-6789AB-CDEF01"
  export_project_id  = "example-project"
  export_dataset_id  = "billing_export"
}
