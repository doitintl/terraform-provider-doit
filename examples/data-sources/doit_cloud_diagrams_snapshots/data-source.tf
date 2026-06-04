# Discover a layer ID from the schemes endpoint.
# Each diagram has layers (statussheet entries) with an "ssid" field
# that serves as the layer ID for other Cloud Diagram data sources.
data "doit_cloud_diagrams_schemes" "all" {}

locals {
  first_scheme_key = keys(data.doit_cloud_diagrams_schemes.all.scheme)[0]
  first_layer_id   = data.doit_cloud_diagrams_schemes.all.scheme[local.first_scheme_key].statussheet[0].ssid
}

# Look up snapshots for the discovered layer.
data "doit_cloud_diagrams_snapshots" "example" {
  id = local.first_layer_id
}
