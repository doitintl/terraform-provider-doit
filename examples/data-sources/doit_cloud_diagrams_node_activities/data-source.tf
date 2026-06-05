# Use case: Find a cloud resource and view its recent activity history.

# Step 1: Search for a specific component by name.
data "doit_cloud_diagrams_search" "lookup" {
  query = "my-production-instance"
}

# Step 2: Pick the first node-type component from search results.
locals {
  nodes    = [for c in data.doit_cloud_diagrams_search.lookup.component : c if c.type == "node"]
  target   = local.nodes[0]
  layer_id = local.target.ss_id
  node_id  = local.target._id
}

# Step 3: Fetch the activity history for that node.
data "doit_cloud_diagrams_node_activities" "history" {
  ss_id   = local.layer_id
  node_id = local.node_id
}

output "activities" {
  value = [for a in data.doit_cloud_diagrams_node_activities.history.cloud_diagrams_node_activities : {
    id        = a._id
    activity  = a.activity
    timestamp = a.timestamp
    user      = a.user
  }]
}
