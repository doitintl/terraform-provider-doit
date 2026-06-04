# Step 1: Discover all diagrams.
data "doit_cloud_diagrams_schemes" "overview" {}

# Step 2: Pick the first diagram and its first layer.
locals {
  first_scheme_key = keys(data.doit_cloud_diagrams_schemes.overview.scheme)[0]
  first_layer_id   = data.doit_cloud_diagrams_schemes.overview.scheme[local.first_scheme_key].statussheet[0].ssid
}

# Step 3: Load component data for that layer via schemes.
data "doit_cloud_diagrams_schemes" "with_components" {
  layer_ids = [local.first_layer_id]
}

# Step 4: Extract node component IDs.
locals {
  ss_data  = data.doit_cloud_diagrams_schemes.with_components.statussheet[local.first_layer_id]
  node_ids = local.ss_data != null && local.ss_data.node != null ? keys(local.ss_data.node) : []
}

# Step 5: Fetch full component details.
data "doit_cloud_diagrams_statussheet" "example" {
  id       = local.first_layer_id
  node_ids = local.node_ids
}

output "nodes" {
  value = data.doit_cloud_diagrams_statussheet.example.node
}
