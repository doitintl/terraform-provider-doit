# Retrieve all anomalies
data "doit_anomalies" "all" {}

# Filter by severity
data "doit_anomalies" "critical" {
  filter = "severityLevel:critical"
}

# Output anomaly details
output "total_anomalies" {
  value = data.doit_anomalies.all.row_count
}

output "anomaly_summary" {
  value = [for a in data.doit_anomalies.all.anomalies : {
    id          = a.id
    service     = a.service_name
    cost_impact = a.cost_of_anomaly
    severity    = a.severity_level
    status      = a.status
  }]
}
