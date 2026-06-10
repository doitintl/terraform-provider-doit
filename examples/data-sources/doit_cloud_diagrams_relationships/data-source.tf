# Retrieve resource relationships for a Cloud Diagram component.
# This example searches for a node and walks both edge and group-membership
# relationships one hop out from it.

data "doit_cloud_diagrams_search" "lookup" {
  query = "peer"
}

locals {
  # Pick the first node-type component from search results.
  nodes     = [for c in data.doit_cloud_diagrams_search.lookup.component : c if c.type == "node"]
  layer_id  = local.nodes[0].ss_id
  anchor_id = local.nodes[0]._id
}

data "doit_cloud_diagrams_relationships" "example" {
  id        = local.layer_id
  rid       = local.anchor_id
  direction = "both"
  depth     = "direct"
  kind      = "both"
}

output "anchor_name" {
  value = data.doit_cloud_diagrams_relationships.example.anchor.name
}

output "relations_count" {
  value = length(data.doit_cloud_diagrams_relationships.example.relations)
}

output "truncated" {
  value = data.doit_cloud_diagrams_relationships.example.truncated
}
