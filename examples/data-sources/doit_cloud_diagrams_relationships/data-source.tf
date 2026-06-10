# Retrieve resource relationships for a Cloud Diagram component.
# This example discovers a diagram, picks the first node, and walks
# both edge and group-membership relationships one hop out.

data "doit_cloud_diagrams_schemes" "all" {}

locals {
  scheme_key = keys(data.doit_cloud_diagrams_schemes.all.scheme)[0]
  layer_id   = data.doit_cloud_diagrams_schemes.all.scheme[local.scheme_key].statussheet[0].ssid
}

data "doit_cloud_diagrams_statussheet" "layer" {
  id = local.layer_id
}

locals {
  anchor_id = keys(data.doit_cloud_diagrams_statussheet.layer.node)[0]
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
