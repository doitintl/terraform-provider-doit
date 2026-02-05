# Retrieve an alert by its ID
data "doit_alert" "example" {
  id = "your-alert-id"
}

# Output the alert details
output "alert_name" {
  value = data.doit_alert.example.name
}

output "alert_recipients" {
  value = data.doit_alert.example.recipients
}

output "alert_last_triggered" {
  value = data.doit_alert.example.last_alerted
}
