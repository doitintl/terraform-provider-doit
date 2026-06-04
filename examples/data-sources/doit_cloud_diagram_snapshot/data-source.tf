# First, discover a layer and its snapshots.
data "doit_cloud_diagram_schemes" "all" {}

locals {
  first_scheme = [for k, v in data.doit_cloud_diagram_schemes.all.scheme : v][0]
  first_layer  = local.first_scheme.statussheet[0]
}

data "doit_cloud_diagram_snapshots" "list" {
  id    = local.first_layer.ssid
  limit = 1
}

locals {
  first_snapshot = tolist(data.doit_cloud_diagram_snapshots.list.cloud_diagram_snapshots)[0]
}

# Get the first snapshot's details.
data "doit_cloud_diagram_snapshot" "example" {
  id          = local.first_layer.ssid
  snapshot_id = local.first_snapshot._id
}

output "snapshot_name" {
  value = data.doit_cloud_diagram_snapshot.example.name
}

output "snapshot_created_at" {
  value = data.doit_cloud_diagram_snapshot.example.created_at
}
