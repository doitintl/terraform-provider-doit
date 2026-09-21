# Retrieve all eligible Slack channels
data "doit_alert_slack_channels" "all" {}

# Filter Slack channels by name substring
data "doit_alert_slack_channels" "production" {
  name_contains = "production"
}

# Paginate Slack channels
data "doit_alert_slack_channels" "limited" {
  max_results = 10
}

# Output Slack channel summary
output "total_channels" {
  value = data.doit_alert_slack_channels.all.row_count
}

output "workspace_status" {
  value = data.doit_alert_slack_channels.all.workspace_status
}

output "is_workspace_connected" {
  value = data.doit_alert_slack_channels.all.is_workspace_connected
}

output "channel_names" {
  value = [for c in data.doit_alert_slack_channels.all.items : c.name]
}

output "channel_details" {
  value = [for c in data.doit_alert_slack_channels.all.items : {
    id        = c.id
    name      = c.name
    type      = c.type
    shared    = c.shared
    workspace = c.workspace
  }]
}
