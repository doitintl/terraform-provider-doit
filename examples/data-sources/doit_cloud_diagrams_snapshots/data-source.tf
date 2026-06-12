# Use case: List snapshots for a specific project's diagram.

# Step 1: Find the diagram by project name.
data "doit_cloud_diagrams_search" "project" {
  query = "my-gcp-project"
}

# Step 2: List snapshots for the matching layer.
data "doit_cloud_diagrams_snapshots" "example" {
  id    = data.doit_cloud_diagrams_search.project.scheme[0].ss_id
  limit = 5
}

output "snapshots" {
  value = [
    for s in data.doit_cloud_diagrams_snapshots.example.cloud_diagrams_snapshots : {
      id         = s._id
      name       = s.name
      created_at = s.created_at
    }
  ]
}
