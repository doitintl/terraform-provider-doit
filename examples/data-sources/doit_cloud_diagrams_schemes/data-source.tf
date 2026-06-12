# Use case: Find and inspect a specific project's diagram.

# Step 1: Search for the diagram by project or account name.
data "doit_cloud_diagrams_search" "project" {
  query = "my-gcp-project"
}

locals {
  layer_id = data.doit_cloud_diagrams_search.project.scheme[0].ss_id
}

# Step 2: Load the diagram with full component data and links.
data "doit_cloud_diagrams_schemes" "project" {
  layer_ids  = [local.layer_id]
  components = true
  link       = true
}

# Summarize what's in the layer.
output "layer_summary" {
  value = {
    nodes  = length(keys(data.doit_cloud_diagrams_schemes.project.statussheet[local.layer_id].node))
    groups = length(keys(data.doit_cloud_diagrams_schemes.project.statussheet[local.layer_id].group))
    links  = length(keys(data.doit_cloud_diagrams_schemes.project.statussheet[local.layer_id].link))
  }
}

# List all diagrams and their layers (overview mode — no components).
data "doit_cloud_diagrams_schemes" "all" {}

output "diagrams" {
  value = {
    for key, diagram in data.doit_cloud_diagrams_schemes.all.scheme :
    key => {
      name   = diagram.name
      type   = diagram.type
      layers = [for layer in diagram.statussheet : layer.account_name]
    }
  }
}

# --- Mermaid flowchart export (community recipe) ---
#
# NOTE: This example is provided as-is to demonstrate what is possible
# with the data source. It is not an officially supported feature and
# may require adjustments for your specific diagram topology.
#
# Generates a Mermaid flowchart from nodes and links that you can paste
# into any Markdown renderer or https://mermaid.live.
#
# Find your layer ID in the Cloud Diagrams URL:
#   console.doit.com/cloud-diagrams/diagram/{scheme_id}/{layer_id}/...

variable "layer_id" {
  description = "Cloud Diagram layer (statussheet) ID."
  type        = string
  default     = "your-layer-id"
}

data "doit_cloud_diagrams_schemes" "mermaid" {
  layer_ids  = [var.layer_id]
  components = true
  link       = true
}

locals {
  ss = data.doit_cloud_diagrams_schemes.mermaid.statussheet[var.layer_id]

  node_lines = [
    for id, n in local.ss.node :
    "  ${id}[\"${replace(coalesce(n.name, id), "\"", "#quot;")}\"]"
  ]

  edge_lines = [
    for id, l in local.ss.link :
    l.connection_type != null
    ? "  ${l.origin._id} -->|${l.connection_type}| ${l.destination._id}"
    : "  ${l.origin._id} --> ${l.destination._id}"
  ]

  mermaid = join("\n", concat(
    ["flowchart LR"],
    local.node_lines,
    local.edge_lines,
  ))
}

output "mermaid" {
  description = "Mermaid flowchart of the diagram layer."
  value       = local.mermaid
}
