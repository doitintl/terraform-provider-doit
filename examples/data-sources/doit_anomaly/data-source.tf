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

# ─────────────────────────────────────────────────────────────────────────────
# Check acknowledgment status
# ─────────────────────────────────────────────────────────────────────────────

output "anomaly_acknowledged" {
  description = "Whether this anomaly has been reviewed"
  value       = data.doit_anomaly.example.acknowledged
}

output "anomaly_acknowledged_at" {
  description = "When the anomaly was first acknowledged (RFC 3339)"
  value       = data.doit_anomaly.example.acknowledged_at
}

output "anomaly_acknowledged_by" {
  description = "Email of the user who acknowledged the anomaly"
  value       = data.doit_anomaly.example.acknowledged_by
}
