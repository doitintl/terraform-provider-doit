terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

# Look up the ID of an existing custom metric. Replace this with the ID of a
# doit_custom_metric resource, or with a fixed ID. Preset metrics are not found.
data "doit_custom_metrics" "share" {
  name_contains = "share"
}

# Read one custom metric by ID.
data "doit_custom_metric" "example" {
  id = data.doit_custom_metrics.share.items[0].id
}

output "custom_metric_formula" {
  value = data.doit_custom_metric.example.formula
}

# The letters (A to E) that the formula uses, with the allocation behind each
output "custom_metric_variables" {
  value = {
    for v in data.doit_custom_metric.example.variables : v.letter => v.allocation_id
  }
}
