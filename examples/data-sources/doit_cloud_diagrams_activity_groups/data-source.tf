# Discover the first available layer ID from the schemes endpoint,
# then list its activity groups.
data "doit_cloud_diagrams_schemes" "discovery" {}

locals {
  first_scheme_key = keys(data.doit_cloud_diagrams_schemes.discovery.scheme)[0]
  first_layer_id   = data.doit_cloud_diagrams_schemes.discovery.scheme[local.first_scheme_key].statussheet[0].ssid
}

data "doit_cloud_diagrams_activity_groups" "example" {
  ss_id = local.first_layer_id
  limit = 5
}

output "activity_groups" {
  value = [
    for g in data.doit_cloud_diagrams_activity_groups.example.cloud_diagrams_activity_groups :
    {
      id        = g._id
      timestamp = g.timestamp
      snapshot  = g.snapshot
      items     = length(g.items)
    }
  ]
}
