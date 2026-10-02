# Create an allocation for the development environment based on a project label.
# anomaly_detection is only valid on single allocations (those using "rule");
# setting it alongside "rules" is rejected at plan time.
resource "doit_allocation" "allocation_dev" {
  name              = "Dev"
  description       = "Development Environment"
  anomaly_detection = true
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

# Create a group allocation from separate single allocations.
# Group allocations use "rules" (plural) and require "unallocated_costs"
# to label costs that don't match any rule. Define each member as its own
# single allocation and reference it with action = "select", so every member
# is a Terraform-managed resource. (Inline rules with action = "create" produce
# the same single allocations in the DoiT API, but they are not Terraform
# resources of their own, so settings like anomaly_detection can't be managed.)
resource "doit_allocation" "region_us" {
  name        = "US"
  description = "Costs in the US"
  rule = {
    formula = "A"
    components = [{
      key    = "country"
      mode   = "is"
      type   = "fixed"
      values = ["US"]
    }]
  }
}

resource "doit_allocation" "region_europe" {
  name        = "Europe"
  description = "Costs in Germany, France, the UK and the Netherlands"
  rule = {
    formula = "A"
    components = [{
      key    = "country"
      mode   = "is"
      type   = "fixed"
      values = ["DE", "FR", "GB", "NL"]
    }]
  }
}

resource "doit_allocation" "allocation_by_region" {
  name              = "By Region"
  description       = "Group costs by region"
  unallocated_costs = "Other Regions"
  rules = [
    { action = "select", id = doit_allocation.region_us.id },
    { action = "select", id = doit_allocation.region_europe.id },
  ]
}

# ─────────────────────────────────────────────────────────────────────────────
# Organizing allocations in folders
# ─────────────────────────────────────────────────────────────────────────────
# Use folder_id to place an allocation inside a Cloud Analytics folder.

resource "doit_folder" "cost_allocations" {
  name = "Cost Allocations"
}

resource "doit_allocation" "in_folder" {
  name        = "Production"
  description = "Production environment costs"
  folder_id   = doit_folder.cost_allocations.id
  rule = {
    formula = "A"
    components = [{
      mode   = "is"
      type   = "project_label"
      key    = "env"
      values = ["prod"]
    }]
  }
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

# ─────────────────────────────────────────────────────────────────────────────
# Nested allocation rules (referencing existing allocations)
# ─────────────────────────────────────────────────────────────────────────────
# Use type = "allocation_rule" to compose allocations from other existing
# allocations. This supports up to 3 levels of nesting depth and
# circular references are not allowed.

# A base allocation for development costs
resource "doit_allocation" "dev" {
  name        = "Development"
  description = "All development environment costs"
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

# A nested allocation that narrows down to dev costs in the US only
resource "doit_allocation" "dev_us" {
  name        = "Development US"
  description = "Development costs in the US (references the Development allocation)"
  rule = {
    formula = "A AND B"
    components = [
      {
        key    = "allocation_rule"
        mode   = "is"
        type   = "allocation_rule"
        values = [doit_allocation.dev.id]
      },
      {
        key    = "country"
        mode   = "is"
        type   = "fixed"
        values = ["US"]
      }
    ]
  }
}

# ─────────────────────────────────────────────────────────────────────────────
# Using doit_dimension (singular) to discover valid component values
# ─────────────────────────────────────────────────────────────────────────────
# While doit_dimensions (plural) helps look up dimension types,
# doit_dimension (singular) retrieves the *values* for a specific dimension.
# This lets you dynamically populate component values instead of hardcoding them.

# Look up valid country values from the API
data "doit_dimension" "country" {
  type = "fixed"
  id   = "country"
}

# Create one single allocation per country group, using values from the API,
# and combine them into a group allocation with action = "select"
locals {
  country_groups = {
    "US Countries"     = ["US"]
    "Europe Countries" = ["DE", "FR", "GB", "NL"]
  }
}

resource "doit_allocation" "country_group" {
  for_each = local.country_groups

  name        = each.key
  description = "Costs in ${join(", ", each.value)}"
  rule = {
    formula = "A"
    components = [{
      key  = "country"
      mode = "is"
      type = "fixed"
      # Use values from the API — filter to the ones we want
      values = [for v in data.doit_dimension.country.values : v.value if contains(each.value, v.value)]
    }]
  }
}

resource "doit_allocation" "dynamic_countries" {
  name              = "Dynamic Countries"
  description       = "Group costs using country values discovered from the API"
  unallocated_costs = "Other Countries"
  rules = [for a in doit_allocation.country_group : {
    action = "select"
    id     = a.id
  }]
}

# ─────────────────────────────────────────────────────────────────────────────
# Allocation factory: one allocation per team, plus a group view of all teams
# ─────────────────────────────────────────────────────────────────────────────
# Define teams once and derive every allocation from the map. Each team gets a
# single allocation with anomaly detection enabled. The group allocation
# selects those singles (action = "select"), so every member of the group is
# a Terraform-managed resource that budgets, alerts and reports can reference.

locals {
  teams = {
    payments = { label = "payments" }
    search   = { label = "search" }
    platform = { label = "platform" }
  }
}

resource "doit_allocation" "team" {
  for_each = local.teams

  name              = "Team: ${each.key}"
  description       = "All costs labeled owner=${each.value.label}"
  anomaly_detection = true
  rule = {
    formula = "A"
    components = [{
      type   = "label"
      key    = "owner"
      mode   = "is"
      values = [each.value.label]
    }]
  }
}

resource "doit_allocation" "by_team" {
  name              = "Cost by Team"
  description       = "Team allocations combined into one group"
  unallocated_costs = "Unowned"
  rules = [for team in doit_allocation.team : {
    action = "select"
    id     = team.id
  }]
}
