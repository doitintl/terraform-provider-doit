# Look up available Cloud Diagrams to find a layer ID.
data "doit_cloud_diagrams_schemes" "all" {}

locals {
  first_scheme_key = keys(data.doit_cloud_diagrams_schemes.all.scheme)[0]
  first_layer_id   = data.doit_cloud_diagrams_schemes.all.scheme[local.first_scheme_key].statussheet[0].ssid
}

# Export the diagram layer with anonymized component IDs.
data "doit_cloud_diagrams_export" "example" {
  id = local.first_layer_id
}

output "export_nodes" {
  value = data.doit_cloud_diagrams_export.example.nodes
}
