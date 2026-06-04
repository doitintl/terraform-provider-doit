# First, discover available layers via schemes.
data "doit_cloud_diagram_schemes" "all" {}

# Pick the first layer of the first diagram.
locals {
  first_scheme = [for k, v in data.doit_cloud_diagram_schemes.all.scheme : v][0]
  first_layer  = local.first_scheme.statussheet[0]
}

# Get all components of that layer.
data "doit_cloud_diagram_statussheet" "example" {
  id = local.first_layer.ssid
}

# Output node names.
output "nodes" {
  value = {
    for id, node in data.doit_cloud_diagram_statussheet.example.node :
    id => node.name
  }
}
