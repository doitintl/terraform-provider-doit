# Look up snapshots for a specific Cloud Diagram layer
data "doit_cloud_diagram_snapshots" "example" {
  id = "your-cloud-diagram-layer-id"
}

# Look up snapshots with pagination and custom sorting
data "doit_cloud_diagram_snapshots" "example_paginated" {
  id     = "your-cloud-diagram-layer-id"
  limit  = 10
  offset = 0
  sort   = "-createdAt"
}
