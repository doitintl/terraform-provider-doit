# Create a report with multiple metrics
resource "doit_report" "my_report" {
  name        = "Monthly Cost Report"
  description = "Tracks monthly costs and usage across cloud providers"
  config = {
    # Use metrics list (supports 1-4 metrics)
    metrics = [
      {
        type  = "basic"
        value = "cost"
      },
      {
        type  = "basic"
        value = "usage"
      }
    ]
    include_promotional_credits = false
    advanced_analysis = {
      trending_up   = false
      trending_down = false
      not_trending  = false
      forecast      = false
    }
    aggregation   = "total"
    time_interval = "month"
    data_source   = "billing"
    dimensions = [
      {
        id   = "year"
        type = "datetime"
      },
      {
        id   = "month"
        type = "datetime"
      }
    ]
    time_range = {
      mode            = "last"
      amount          = 12
      include_current = true
      unit            = "month"
    }
    # Compare against the same period last year
    secondary_time_range = {
      amount          = 1
      unit            = "year"
      include_current = false
    }
    filters = [
      {
        inverse = false
        id      = "cloud_provider"
        type    = "fixed"
        mode    = "is"
        values  = ["amazon-web-services", "google-cloud"]
      }
    ]
    group = [
      {
        id   = "cloud_provider"
        type = "fixed"
      }
    ]
    layout         = "table"
    display_values = "actuals_only"
    currency       = "USD"
  }
}

# ─────────────────────────────────────────────────────────────────────────────
# Discovering valid filter values using data sources
# ─────────────────────────────────────────────────────────────────────────────
# Users often need to know which values are accepted in report filter attributes.
# The following examples show how to use doit_platforms, doit_products, and
# doit_dimensions data sources to discover valid values dynamically.

# Use doit_products to discover valid service_description filter values.
# Each product has an `id`, a `display_name`, and a `platform`.
# Setting `platform` filters products to a specific cloud provider.
#
# To discover valid platform identifiers for the `platform` attribute,
# use the doit_platforms data source:
data "doit_platforms" "all" {}

output "available_platforms" {
  description = "Available platform IDs for use with the doit_products data source"
  value       = [for p in data.doit_platforms.all.platforms : { id = p.id, name = p.display_name }]
}

# Get products for Google Cloud Platform
data "doit_products" "gcp" {
  platform = "google_cloud_platform"
}

output "gcp_products" {
  description = "Available GCP product IDs that can be used in service_description filters"
  value       = [for p in data.doit_products.gcp.products : { id = p.id, name = p.display_name }]
}

# Create a report filtered by specific GCP products from the data source
resource "doit_report" "gcp_product_costs" {
  name        = "GCP Product Costs (Dynamic)"
  description = "Uses doit_products data source to populate service_description filter values"
  config = {
    metrics        = [{ type = "basic", value = "cost" }]
    aggregation    = "total"
    data_source    = "billing"
    time_interval  = "month"
    display_values = "actuals_only"
    time_range = {
      mode            = "last"
      amount          = 3
      include_current = true
      unit            = "month"
    }
    # Use product IDs from the data source as filter values
    filters = [
      {
        id     = "service_description"
        type   = "fixed"
        mode   = "is"
        values = [for p in data.doit_products.gcp.products : p.id]
      }
    ]
    group = [
      { id = "cloud_provider", type = "fixed" },
      { id = "service_description", type = "fixed" }
    ]
    layout   = "table"
    currency = "USD"
  }
}

# Use doit_dimensions to discover valid dimension IDs and types for use in
# report filters, group-by, and dimension fields.
# Each dimension has an `id`, `type` (e.g. "fixed", "optional"), and `label`.
# These map directly to filters[].id, filters[].type, group[].id, group[].type.
data "doit_dimensions" "all" {}

output "available_dimensions" {
  description = "Available dimension IDs and types for use in report filters and group-by"
  value       = [for d in data.doit_dimensions.all.dimensions : { id = d.id, type = d.type, label = d.label }]
}

# Build a lookup map from dimension ID to its type for easy referencing.
# Some dimension IDs may exist with multiple types, so we group by ID and
# take the first type.
locals {
  dimension_types = { for id, types in {
    for d in data.doit_dimensions.all.dimensions : d.id => d.type...
  } : id => types[0] }
}

resource "doit_report" "cost_by_region" {
  name        = "Cost by Region (Dimension Discovery)"
  description = "Uses doit_dimensions data source to dynamically look up dimension types"
  config = {
    metrics        = [{ type = "basic", value = "cost" }]
    aggregation    = "total"
    data_source    = "billing"
    time_interval  = "month"
    display_values = "actuals_only"
    time_range = {
      mode            = "last"
      amount          = 3
      include_current = true
      unit            = "month"
    }
    # Use the dimension lookup to get the correct type for each dimension ID
    group = [
      {
        id   = "region"
        type = local.dimension_types["region"]
      }
    ]
    layout   = "table"
    currency = "USD"
  }
}
