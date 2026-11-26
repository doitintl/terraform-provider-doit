data "doit_cloudconnect_gcp_requirements" "project" {
  scope_kind  = "gcp_project"
  project_id  = "example-project"
  feature_ids = ["core", "bigquery-finops"]

  feature_inputs = {
    bigquery_finops = {
      location = "EU"
    }
  }
}

# The permissions each feature needs on the project
output "feature_permissions" {
  value = data.doit_cloudconnect_gcp_requirements.project.features
}
