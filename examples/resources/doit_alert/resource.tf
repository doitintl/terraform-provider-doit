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
