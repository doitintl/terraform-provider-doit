# Retrieve all accessible diagrams (overview mode — no components).
data "doit_cloud_diagrams_schemes" "all" {}

# Output diagram names and their layers.
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

# Retrieve specific diagrams with full component data.
data "doit_cloud_diagrams_schemes" "detailed" {
  scheme_ids = ["diagram-id-1", "diagram-id-2"]
  components = true
  skip_empty = true
}

# Retrieve specific layers with components and links.
data "doit_cloud_diagrams_schemes" "layers" {
  layer_ids  = ["layer-id-1"]
  components = true
  link       = true
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
