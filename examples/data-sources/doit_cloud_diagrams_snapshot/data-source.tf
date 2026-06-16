# Use case: Get details for a specific snapshot of a project's diagram.

# Step 1: Find the diagram by project name.
data "doit_cloud_diagrams_search" "project" {
  query = "my-gcp-project"
}

locals {
  layer_id = data.doit_cloud_diagrams_search.project.scheme[0].ss_id
}

# Step 2: List snapshots and pick the most recent one.
data "doit_cloud_diagrams_snapshots" "list" {
  id    = local.layer_id
  limit = 1
}

locals {
  latest_snapshot = tolist(data.doit_cloud_diagrams_snapshots.list.cloud_diagrams_snapshots)[0]
}

# Step 3: Get the snapshot's full details.
data "doit_cloud_diagrams_snapshot" "latest" {
  id          = local.layer_id
  snapshot_id = local.latest_snapshot._id
}

output "snapshot_name" {
  value = data.doit_cloud_diagrams_snapshot.latest.name
}

output "snapshot_created_at" {
  value = data.doit_cloud_diagrams_snapshot.latest.created_at
}
