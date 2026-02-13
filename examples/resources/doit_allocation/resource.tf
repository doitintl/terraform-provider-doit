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
