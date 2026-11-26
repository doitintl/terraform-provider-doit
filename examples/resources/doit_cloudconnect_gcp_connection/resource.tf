# Connect a project with the core feature
resource "doit_cloudconnect_gcp_connection" "project" {
  scope_kind  = "gcp_project"
  project_id  = "example-project"
  feature_ids = ["core"]

  service_account = {
    email      = "doit-connect@example-project.iam.gserviceaccount.com"
    unique_id  = "100000000000000000001"
    project_id = "example-project"
  }
}

# Connect an organization with BigQuery FinOps, storing its logs in the EU
resource "doit_cloudconnect_gcp_connection" "organization" {
  scope_kind      = "gcp_organization"
  organization_id = "123456789012"
  feature_ids     = ["core", "bigquery-finops"]

  service_account = {
    email      = "doit-connect@example-admin-project.iam.gserviceaccount.com"
    unique_id  = "100000000000000000002"
    project_id = "example-admin-project"
  }

  feature_inputs = {
    bigquery_finops = {
      location = "EU"
    }
  }
}
