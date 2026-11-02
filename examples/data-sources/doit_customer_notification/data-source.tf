terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

variable "notification_id" {
  type        = string
  description = "The ID of an organization notification, for example from the doit_customer_notifications data source."
}

# Read one organization-wide cost-anomaly notification. Requires the
# "Manage settings" permission.
data "doit_customer_notification" "example" {
  id = var.notification_id
}

output "notification_name" {
  value = data.doit_customer_notification.example.name
}

output "notification_emails" {
  value = data.doit_customer_notification.example.destinations.emails
}

output "min_severity" {
  value = data.doit_customer_notification.example.settings.cost_anomalies.min_severity
}
