# List recent cloud incidents (max_results recommended to avoid long fetch times)
data "doit_cloud_incidents" "recent" {
  max_results = 100
}

# Filter cloud incidents by platform
data "doit_cloud_incidents" "gcp_only" {
  max_results = 50
  filter      = "platform:google-cloud"
}

# Paginate through cloud incidents
data "doit_cloud_incidents" "page1" {
  max_results = 25
}

# Use the page_token from the previous data source to get the next page
data "doit_cloud_incidents" "page2" {
  max_results = 25
  page_token  = data.doit_cloud_incidents.page1.page_token
}

output "incident_count" {
  value = data.doit_cloud_incidents.recent.row_count
}
