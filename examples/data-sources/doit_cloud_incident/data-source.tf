# Retrieve a cloud incident by its ID
data "doit_cloud_incident" "example" {
  id = "iWC3OgbshntQJZFHbpRT"
}

# Output cloud incident details
output "incident_title" {
  value = data.doit_cloud_incident.example.title
}

output "incident_platform" {
  value = data.doit_cloud_incident.example.platform
}

output "incident_product" {
  value = data.doit_cloud_incident.example.product
}

output "incident_status" {
  value = data.doit_cloud_incident.example.status
}

output "incident_summary" {
  value = data.doit_cloud_incident.example.summary
}
