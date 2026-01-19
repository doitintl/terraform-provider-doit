# List all cloud incidents
data "doit_cloud_incidents" "all" {
}

# Filter cloud incidents by platform
data "doit_cloud_incidents" "gcp_only" {
  filter = "platform:google-cloud"
}

output "incident_count" {
  value = data.doit_cloud_incidents.all.row_count
}
