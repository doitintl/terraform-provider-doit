# Use doit_current_user to dynamically get the current user's email
# for alert recipients instead of hardcoding email addresses
data "doit_current_user" "me" {}

# Create an alert that triggers when costs exceed $1000 per month
resource "doit_alert" "cost_alert" {
  name = "Monthly Cost Alert"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 1000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
  }
  recipients = [data.doit_current_user.me.email]
}

# Alert with scope filters
resource "doit_alert" "aws_cost_alert" {
  name = "AWS Cost Alert"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "day"
    value         = 100
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    scopes = [
      {
        type   = "fixed"
        id     = "cloud_provider"
        mode   = "is"
        values = ["amazon-web-services"]
      }
    ]
  }
  recipients = [data.doit_current_user.me.email]
}

# ─────────────────────────────────────────────────────────────────────────────
# Discovering valid scope values using data sources
# ─────────────────────────────────────────────────────────────────────────────
# Alert scopes use the same id/type/mode/values structure as report filters.
# Use doit_products, doit_dimensions, and doit_users to populate these dynamically.

# Use doit_products to scope an alert to specific cloud services
data "doit_products" "gcp" {
  platform = "google_cloud_platform"
}

resource "doit_alert" "gcp_compute_cost_alert" {
  name = "GCP Compute Cost Alert"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "day"
    value         = 500
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    # Use product IDs from the data source as scope filter values
    scopes = [
      {
        type   = "fixed"
        id     = "service_description"
        mode   = "is"
        values = [for p in data.doit_products.gcp.products : p.id if p.display_name == "Compute Engine"]
      }
    ]
  }
  recipients = [data.doit_current_user.me.email]
}

# Use doit_dimensions to look up correct dimension types for scope fields
data "doit_dimensions" "all" {}

locals {
  dimension_types = { for id, types in {
    for d in data.doit_dimensions.all.dimensions : d.id => d.type...
  } : id => types[0] }
}

resource "doit_alert" "region_cost_alert" {
  name = "Region Cost Alert"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 5000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    # Use the dimension lookup to get the correct type for each scope field
    scopes = [
      {
        id     = "region"
        type   = local.dimension_types["region"]
        mode   = "is"
        values = ["us-east1", "us-central1"]
      }
    ]
  }
  recipients = [data.doit_current_user.me.email]
}

# ─────────────────────────────────────────────────────────────────────────────
# Using doit_dimension (singular) to discover valid scope values
# ─────────────────────────────────────────────────────────────────────────────
# While doit_dimensions (plural) helps look up dimension types,
# doit_dimension (singular) retrieves the *values* for a specific dimension.
# This lets you dynamically populate scope values instead of hardcoding them.

# Look up valid cloud_provider values from the API
data "doit_dimension" "cloud_provider" {
  type = "fixed"
  id   = "cloud_provider"
}

resource "doit_alert" "all_clouds_cost_alert" {
  name = "All Clouds Cost Alert"
  config = {
    metric = {
      type  = "basic"
      value = "cost"
    }
    time_interval = "month"
    value         = 10000
    currency      = "USD"
    condition     = "value"
    operator      = "gt"
    # Scope values discovered dynamically from the API
    scopes = [
      {
        id     = "cloud_provider"
        type   = "fixed"
        mode   = "is"
        values = [for v in data.doit_dimension.cloud_provider.values : v.value]
      }
    ]
  }
  recipients = [data.doit_current_user.me.email]
}

# ─────────────────────────────────────────────────────────────────────────────
# Percentage-change alerts without the noise, posted to Slack
# ─────────────────────────────────────────────────────────────────────────────
# A percentage-change alert on a small daily spend fires on every blip
# ("+400%" on $2). ignore_values_range skips evaluations while the metric value
# is within the bounds. The bounds use metric units (here USD per day), not
# percent, and are only valid with condition = "percentage-change".

resource "doit_alert" "team_spike" {
  for_each = toset(["payments", "search", "platform"])

  name = "Team ${each.key}: daily cost spike"
  config = {
    metric        = { type = "basic", value = "cost" }
    time_interval = "day"
    condition     = "percentage-change"
    operator      = "gt"
    value         = 30 # percent
    currency      = "USD"
    ignore_values_range = {
      lower_bound = 0
      upper_bound = 50 # USD per day
    }
    # Evaluate each service separately, so one service spiking is enough
    evaluate_for_each = "fixed:service_description"
    scopes = [{
      type   = "label"
      id     = "owner"
      mode   = "is"
      values = [each.key]
    }]
  }
  recipients = [data.doit_current_user.me.email]
  # A shared Slack channel only needs its ID. A channel in a connected
  # workspace also needs workspace = "<workspace name>" instead of shared.
  recipients_slack_channels = [{
    id     = "C0123456789"
    shared = true
  }]
}
