# Use case: View recent infrastructure changes for a specific project.

# Step 1: Find the diagram by project name.
data "doit_cloud_diagrams_search" "project" {
  query = "my-gcp-project"
}

# Step 2: Fetch recent activity groups for the matching layer.
data "doit_cloud_diagrams_activity_groups" "recent" {
  ss_id = data.doit_cloud_diagrams_search.project.scheme[0].ss_id
  limit = 3
}

# Each activity group bundles related changes that happened together.
# The items inside each group describe what actually changed.
output "recent_changes" {
  value = [
    for g in data.doit_cloud_diagrams_activity_groups.recent.cloud_diagrams_activity_groups : {
      timestamp = g.timestamp
      changes = [
        for item in g.items : {
          activity     = item.activity
          service_type = item.service_type
          timestamp    = item.timestamp
        }
      ]
    }
  ]
}
