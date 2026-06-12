# Use case: Find a specific VPC and fetch its full component details.

# Step 1: Search for the component by name.
data "doit_cloud_diagrams_search" "vpc" {
  query = "my-production-vpc"
}

locals {
  target   = data.doit_cloud_diagrams_search.vpc.component[0]
  layer_id = local.target.ss_id
}

# Step 2: Fetch the component with all its properties.
# Use node_ids, element_ids, group_ids, etc. to request specific components.
data "doit_cloud_diagrams_statussheet" "vpc_details" {
  id       = local.layer_id
  node_ids = [local.target._id]
}

output "vpc_details" {
  value = data.doit_cloud_diagrams_statussheet.vpc_details.node[local.target._id]
}

# Use case: Fetch multiple component types at once.
# You can combine different ID lists in a single call.
data "doit_cloud_diagrams_statussheet" "mixed" {
  id          = local.layer_id
  node_ids    = [local.target._id]
  group_ids   = [local.target._id]
  link_ids    = [local.target._id]
}

output "mixed_results" {
  value = {
    nodes  = length(keys(data.doit_cloud_diagrams_statussheet.mixed.node))
    groups = length(keys(data.doit_cloud_diagrams_statussheet.mixed.group))
    links  = length(keys(data.doit_cloud_diagrams_statussheet.mixed.link))
  }
}
