# Retrieve a cost anomaly by its ID
data "doit_anomaly" "example" {
  id = "5c981868-e44a-4e57-93ad-8d5c1cff91fe"
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
