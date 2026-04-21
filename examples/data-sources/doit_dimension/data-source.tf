# ─────────────────────────────────────────────────────────────────────────────
# Retrieve a single dimension to discover its possible values
# ─────────────────────────────────────────────────────────────────────────────
# Use doit_dimension (singular) when you know the dimension type and ID,
# and want to retrieve its allowed values. This is useful for dynamically
# populating filter "values" fields in reports, budgets, and allocations.

data "doit_dimension" "services" {
  type = "fixed"
  id   = "service_description"
}

output "dimension_label" {
  value = data.doit_dimension.services.label
}

output "service_values" {
  value = [for v in data.doit_dimension.services.values : v.value]
}

# ─────────────────────────────────────────────────────────────────────────────
# Combining doit_dimensions (list) and doit_dimension (singular)
# ─────────────────────────────────────────────────────────────────────────────
# Use doit_dimensions (plural) to discover available dimensions, then
# doit_dimension (singular) to look up the valid values of a specific one.
# Together they replace hardcoded filter values with dynamic lookups.

# Step 1: List all dimensions to discover what's available
data "doit_dimensions" "all" {}

output "available_dimensions" {
  value = [for d in data.doit_dimensions.all.dimensions : {
    id    = d.id
    type  = d.type
    label = d.label
  }]
}

# Step 2: Look up the specific dimension to get its valid values
data "doit_dimension" "cloud_provider" {
  type = "fixed"
  id   = "cloud_provider"
}

# Step 3: Use the values in a resource (e.g., a budget scope or report filter)
output "available_clouds" {
  description = "Valid cloud_provider filter values, discovered dynamically"
  value       = [for v in data.doit_dimension.cloud_provider.values : v.value]
}
