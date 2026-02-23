# Create an allocation for the development environment based on a project label
resource "doit_allocation" "allocation_dev" {
  name        = "Dev"
  description = "Development Environment"
  rule = {
    formula = "A"
    components = [{
      mode   = "is"
      type   = "project_label"
      key    = "env"
      values = ["dev"]
    }]
  }
}

# Create an allocation for your dev GKE clusters in the US
resource "doit_allocation" "allocation_dev_clusters_us" {
  name        = "Dev Clusters US"
  description = "Development GKE Clusters in the US"
  rule = {
    formula = "A AND B"
    components = [
      {
        include_null = true
        mode         = "is"
        type         = "fixed"
        key          = "kubernetes_cluster_name"
        values       = ["dev"]
      },
      {
        key  = "country"
        mode = "is"
        type = "fixed"
        values = [
          "US",
        ]
      }
    ]
  }
}

# Create a group allocation that combines multiple rules.
# Group allocations use "rules" (plural) and require "unallocated_costs"
# to label costs that don't match any rule.
resource "doit_allocation" "allocation_by_region" {
  name              = "By Region"
  description       = "Group costs by region"
  unallocated_costs = "Other Regions"
  rules = [
    {
      action  = "create"
      name    = "US"
      formula = "A"
      components = [{
        key    = "country"
        mode   = "is"
        type   = "fixed"
        values = ["US"]
      }]
    },
    {
      action  = "create"
      name    = "Europe"
      formula = "A"
      components = [{
        key    = "country"
        mode   = "is"
        type   = "fixed"
        values = ["DE", "FR", "GB", "NL"]
      }]
    }
  ]
}

# ─────────────────────────────────────────────────────────────────────────────
# Discovering valid component values using data sources
# ─────────────────────────────────────────────────────────────────────────────
# Allocation components use `type` and `key` fields that correspond to dimension
# types and IDs. Use doit_dimensions to discover valid combinations.

data "doit_dimensions" "all" {}

# Build a lookup map from dimension ID to its type
locals {
  dimension_types = { for id, types in {
    for d in data.doit_dimensions.all.dimensions : d.id => d.type...
  } : id => types[0] }
}

# Use doit_products to discover valid service IDs for allocation components
data "doit_products" "gcp" {
  platform = "google_cloud_platform"
}

# Create an allocation scoped to GCP Compute Engine services using data sources
resource "doit_allocation" "gcp_compute" {
  name        = "GCP Compute"
  description = "GCP Compute Engine costs"
  rule = {
    formula = "A"
    components = [{
      mode   = "is"
      type   = local.dimension_types["service_description"]
      key    = "service_description"
      values = [for p in data.doit_products.gcp.products : p.id if p.display_name == "Compute Engine"]
    }]
  }
}
