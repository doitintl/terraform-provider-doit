# Retrieve all alerts
data "doit_alerts" "all" {}

# Filter alerts by name
data "doit_alerts" "aws_alerts" {
  filter = "name:[AWS]"
}

# Sort by last triggered time
data "doit_alerts" "recent" {
  sort_by    = "lastAlerted"
  sort_order = "desc"
}

# Output alert summary
output "total_alerts" {
  value = data.doit_alerts.all.row_count
}

output "alert_names" {
  value = [for a in data.doit_alerts.all.alerts : a.name]
}

output "alert_details" {
  value = [for a in data.doit_alerts.all.alerts : {
    id            = a.id
    name          = a.name
    time_interval = a.config.time_interval
    threshold     = a.config.value
    recipients    = a.recipients
  }]
}
