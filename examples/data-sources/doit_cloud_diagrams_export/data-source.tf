# Use case: Export a specific project's diagram and inspect its topology.

# Step 1: Search for the diagram by project name.
data "doit_cloud_diagrams_search" "project" {
  query = "my-gcp-project"
}

locals {
  # Pick the first matching layer.
  layer_id = data.doit_cloud_diagrams_search.project.scheme[0].ss_id
}

# Step 2: Export the layer (component IDs are anonymized for portability).
data "doit_cloud_diagrams_export" "layer" {
  id = local.layer_id
}

# Summarize the exported topology.
output "topology" {
  value = {
    nodes  = length(data.doit_cloud_diagrams_export.layer.nodes)
    groups = length(data.doit_cloud_diagrams_export.layer.groups)
    links  = length(data.doit_cloud_diagrams_export.layer.links)
  }
}

# List link connections with their origin and destination.
output "connections" {
  value = [
    for l in data.doit_cloud_diagrams_export.layer.links : {
      origin      = l.origin._id
      destination = l.destination._id
      type        = l.connection_type
    }
  ]
}
