# Use case: Find a specific cloud resource and fetch its full component details.

# Step 1: Search for components by name (e.g. a specific EC2 instance or VPC).
data "doit_cloud_diagrams_search" "lookup" {
  query = "my-production-vpc"
}

# Step 2: Collect matching component IDs grouped by type and layer.
locals {
  # Pick the first matching component.
  target     = data.doit_cloud_diagrams_search.lookup.component[0]
  layer_id   = local.target.ss_id
  target_ids = [local.target._id]

  # Map component type to the correct ID list attribute.
  is_node    = local.target.type == "node"
  is_element = local.target.type == "element"
  is_group   = local.target.type == "group"
}

# Step 3: Fetch full component details (all properties and attributes).
data "doit_cloud_diagrams_statussheet" "details" {
  id          = local.layer_id
  node_ids    = local.is_node ? local.target_ids : null
  element_ids = local.is_element ? local.target_ids : null
  group_ids   = local.is_group ? local.target_ids : null
}

output "component_details" {
  value = {
    nodes    = data.doit_cloud_diagrams_statussheet.details.node
    elements = data.doit_cloud_diagrams_statussheet.details.element
    groups   = data.doit_cloud_diagrams_statussheet.details.group
  }
}
