# Retrieve a cost anomaly by its ID
data "doit_anomaly" "example" {
  id = "your-anomaly-id"
}

# Output anomaly details
output "anomaly_platform" {
  value = data.doit_anomaly.example.platform
}

output "anomaly_service" {
  value = data.doit_anomaly.example.service_name
}

output "anomaly_cost" {
  value = data.doit_anomaly.example.cost_of_anomaly
}

output "anomaly_severity" {
  value = data.doit_anomaly.example.severity_level
}
