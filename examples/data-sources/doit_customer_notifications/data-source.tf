terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

# List all organization-wide cost-anomaly notifications (not paginated).
# Requires the "Manage settings" permission.
data "doit_customer_notifications" "all" {}

locals {
  # Notifications that also post to Slack.
  slack_notifications = [
    for n in data.doit_customer_notifications.all.items : n
    if length(n.destinations.slack_channels) > 0
  ]
}

output "notification_names" {
  value = [for n in data.doit_customer_notifications.all.items : n.name]
}

output "slack_notification_ids" {
  value = [for n in local.slack_notifications : n.id]
}
