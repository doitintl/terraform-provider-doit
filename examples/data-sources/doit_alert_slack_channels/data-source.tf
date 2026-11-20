# Retrieve all eligible Slack channels
data "doit_alert_slack_channels" "all" {}

# Filter Slack channels by name substring
data "doit_alert_slack_channels" "production" {
  name_contains = "production"
}

# Read a single page of Slack channels. Channels come in workspace order (shared,
# public, then private), a page can be shorter than max_results or even empty
# while page_token is set, and row_count is null until the whole list is returned.
data "doit_alert_slack_channels" "limited" {
  max_results = 10
}

# Continue with the next page; stop when page_token is null.
data "doit_alert_slack_channels" "limited_next" {
  max_results = 10
  page_token  = data.doit_alert_slack_channels.limited.page_token
}

output "next_page_token" {
  value = data.doit_alert_slack_channels.limited_next.page_token
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

# False when the list may be missing channels: Slack rate limited or failed the lookup,
# the private-channel scope is missing, or the caller is a service account (private
# channels are never listed for service accounts). It does not mean more pages remain.
output "channels_complete" {
  value = data.doit_alert_slack_channels.all.channels_complete
}

# The shared channel can also be listed as a public channel, so an id may appear
# twice (the two entries differ in shared and workspace). Key a for_each on
# id and workspace, or deduplicate with distinct(), instead of on id alone.
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
